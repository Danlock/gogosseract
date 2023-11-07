package gogosseract

import (
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/danlock/pkg/errors"
	"github.com/tetratelabs/wazero"
)

type PoolConfig struct {
	Config
	// TrainingDataBytes is Config.TrainingData, but as a []byte for concurrency's sake.
	// Multiple Tesseract workers can't read from a single io.Reader, so they can't benefit from streaming the data.
	// For convenience you only need to set either Config.TrainingData or TrainingDataBytes.
	TrainingDataBytes []byte
}

// NewPool creates a pool of Tesseract clients for safe, efficient concurrent use.
func NewPool(ctx context.Context, count uint, cfg PoolConfig) (_ *Pool, err error) {
	if count == 0 {
		return nil, errors.New("got zero count")
	}
	if cfg.TrainingDataBytes == nil && cfg.TrainingData == nil {
		return nil, errors.New("requires either PoolConfig.TrainingDataBytes or Config.TrainingData")
	}
	if cfg.TrainingDataBytes == nil {
		cfg.TrainingDataBytes, err = io.ReadAll(cfg.TrainingData)
		if err != nil {
			return nil, errors.Errorf("reading cfg.TrainingData failed because %w", err)
		}
	}
	// Set WASMCache by default to speed up worker compilation
	if cfg.Config.WASMCache == nil {
		cfg.Config.WASMCache = wazero.NewCompilationCache()
	}
	p := &Pool{
		// errChan must be big enough for all workers to fail simultaneously
		errChan: make(chan error, count),
		reqChan: make(chan workerReq),
		cfg:     cfg,
	}
	p.ctx, p.shutdown = context.WithCancelCause(ctx)
	ctx = p.ctx
	for i := uint(0); i < count; i++ {
		p.wg.Add(1)
		// Synchronously startup workers, returning an error on any failure
		go p.runTesseract(ctx)
		select {
		case <-ctx.Done():
			return nil, errors.Errorf("timed out during worker setup due to %w", context.Cause(ctx))
		case err := <-p.errChan:
			if err != nil {
				// Disregard close() errors since err actually caused this.
				// Further errors will just be an effect of the context cancellation.
				_ = p.close(false)
				return nil, errors.Errorf("failed worker setup due to %w", err)
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
	ctx      context.Context
	wg       sync.WaitGroup
	cfg      PoolConfig
	shutdown context.CancelCauseFunc
	errChan  chan error
	reqChan  chan workerReq
}

func (p *Pool) runTesseract(ctx context.Context) (err error) {
	cfg := p.cfg.Config
	cfg.TrainingData = bytes.NewBuffer(p.cfg.TrainingDataBytes)
	tess, err := New(ctx, cfg)
	defer func() {
		if tess != nil {
			err = errors.Join(err, tess.Close(ctx))
		}
		// defer sending on errChan so we don't have to do it each return
		p.errChan <- err
		p.wg.Done()
	}()

	if err != nil {
		return errors.Wrap(err)
	}
	// Send back a nil so NewPool knows this worker's ready to receive requests
	p.errChan <- nil

	for {
		select {
		case <-ctx.Done():
			return nil
		case req := <-p.reqChan:
			if err := tess.LoadImage(req.ctx, req.img, req.opts.LoadImageOptions); err != nil {
				req.respChan <- workerResp{err: errors.Errorf(" %w", err)}
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
	req := workerReq{ctx: ctx, img: img, opts: opts, respChan: make(chan workerResp, 1)}

	select {
	case <-p.ctx.Done():
		return "", errors.Errorf("while waiting for available worker %w", context.Cause(p.ctx))
	case <-ctx.Done():
		return "", errors.Errorf("while waiting for available worker %w", context.Cause(ctx))
	case p.reqChan <- req:
	}
	// with respChan buffered, even if we time out early the worker will send their resp without blocking forever
	select {
	case <-p.ctx.Done():
		return "", errors.Errorf("while waiting for worker's response %w", context.Cause(p.ctx))
	case <-ctx.Done():
		return "", errors.Errorf("while waiting for worker's response %w", context.Cause(ctx))
	case resp := <-req.respChan:
		return resp.str, resp.err
	}
}

// Close shuts down the Pool, Close's the Tesseract workers, and waits for the goroutines to end.
// The returned error is a Join of close errors from every worker, if they exist.
func (p *Pool) Close() error {
	return p.close(true)
}

func (p *Pool) close(getErrors bool) error {
	p.shutdown(errors.New(""))
	p.wg.Wait()
	if !getErrors {
		return nil
	}
	// errChan was made big enough to hold an error or a nil from each worker
	workerErrors := make([]error, cap(p.errChan))
	for i := range workerErrors {
		workerErrors[i] = <-p.errChan
	}
	return errors.Join(workerErrors...)
}
