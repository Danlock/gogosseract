package gogosseract

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/tetratelabs/wazero"
)

type PoolConfig struct {
	Config
}

// NewPool creates a pool of Tesseract clients for safe, efficient concurrent use.
func NewPool(ctx context.Context, count uint, cfg PoolConfig) (*Pool, error) {
	logPrefix := "gogosseract.NewPool"

	if count == 0 {
		return nil, fmt.Errorf(logPrefix + " got zero count")
	}

	// Set WASMCache by default to speed up worker compilation
	if cfg.Config.WASMCache == nil {
		cfg.Config.WASMCache = wazero.NewCompilationCache()
	}

	p := &Pool{
		// errChan must be big enough for all workers to fail simultaenously
		errChan: make(chan error, count),
		reqChan: make(chan workerReq),
		cfg:     cfg,
	}

	ctx, p.shutdown = context.WithCancelCause(ctx)

	for i := uint(0); i < count; i++ {
		p.wg.Add(1)
		// Synchronously startup workers, returning an error on any failure
		go p.runTesseract(ctx)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf(logPrefix+" timed out during worker setup due to %w", context.Cause(ctx))
		case err := <-p.errChan:
			if err != nil {
				// Wait for any previously setup workers to stop before returning this error so we're in a known state
				p.shutdown(err)
				p.wg.Wait()
				return nil, fmt.Errorf(logPrefix+" failed worker setup due to %w", err)
			}
		}
	}

	return p, nil
}

type workerReq struct {
	ctx  context.Context
	img  io.Reader
	opts ParseImageOptions

	respChan chan workerResp
}

type workerResp struct {
	str string
	err error
}

type Pool struct {
	wg       sync.WaitGroup
	cfg      PoolConfig
	shutdown context.CancelCauseFunc
	errChan  chan error
	reqChan  chan workerReq
}

func (p *Pool) runTesseract(ctx context.Context) (err error) {
	logPrefix := "gogosseract.Pool.runTesseract"
	tess, err := New(ctx, p.cfg.Config)
	defer func() {
		if tess != nil {
			err = errors.Join(err, tess.Close(ctx))
		}
		// defer sending on errChan so we don't have to do it each return
		p.errChan <- err
		p.wg.Done()
	}()

	if err != nil {
		return fmt.Errorf(logPrefix+" %w", err)
	}
	// Send back a nil so NewPool knows this worker's ready to receive requests
	p.errChan <- nil

	for {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case req := <-p.reqChan:
			if err := tess.LoadImage(req.ctx, req.img, req.opts.LoadImageOptions); err != nil {
				req.respChan <- workerResp{err: fmt.Errorf(logPrefix+" %w", err)}
				continue
			}
			var resp workerResp
			if req.opts.IsHOCR {
				resp.str, resp.err = tess.GetHOCR(ctx, req.opts.ProgressCB)
			} else {
				resp.str, resp.err = tess.GetText(ctx, req.opts.ProgressCB)
			}
			req.respChan <- resp
			// We could clear the image in advance to release the memory but unfortunately...
			// WASM memory grows but doesn't shrink, so that won't reduce memory usage.
			// The only way to release memory is closing a Tesseract client and creating a new one...
		}
	}
}

type ParseImageOptions struct {
	LoadImageOptions
	// IsHOCR makes a GetHOCR request instead of the default GetText
	IsHOCR bool
	// Called whenever Tesseract's parsing progresses, gives a percentage.
	ProgressCB func(int32)
}

// ParseImage loads an image into our Tesseract object and gets back text from it.
// Both actions are executed on an available worker.
// Set a timeout with context.WithTimeout to handle the case where all workers are busy.
func (p *Pool) ParseImage(ctx context.Context, img io.Reader, opts ParseImageOptions) (string, error) {
	logPrefix := "Pool.ParseImage"
	req := workerReq{ctx: ctx, img: img, opts: opts, respChan: make(chan workerResp, 1)}

	select {
	case <-ctx.Done():
		return "", fmt.Errorf(logPrefix+" while waiting for available worker %w", context.Cause(ctx))
	case p.reqChan <- req:
	}
	// with respChan buffered, even if we time out early the worker can send their text without blocking forever
	select {
	case <-ctx.Done():
		return "", fmt.Errorf(logPrefix+" while waiting for worker's response %w", context.Cause(ctx))
	case resp := <-req.respChan:
		return resp.str, resp.err
	}
}
