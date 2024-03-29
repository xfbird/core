package foundation

import (
	"github.com/pangdogs/core/internal"
	"sync"
	"sync/atomic"
)

type AppWhole interface {
	internal.App
	GetInheritor() internal.App
	AddEntity(entity internal.Entity)
	RemoveEntity(entID uint64)
}

func NewApp(ctx internal.Context, optFuncs ...NewAppOptionFunc) internal.App {
	app := &App{}

	opts := &AppOptions{}
	for _, optFun := range append([]NewAppOptionFunc{NewAppOption.Default()}, optFuncs...) {
		optFun(opts)
	}

	app.InitApp(ctx, opts)

	return app.inheritor
}

type App struct {
	Runnable
	internal.Context
	AppOptions
	uidMaker  uint64
	entityMap sync.Map
}

func (app *App) InitApp(ctx internal.Context, opts *AppOptions) {
	if ctx == nil {
		panic("nil ctx")
	}

	if opts == nil {
		panic("nil opts")
	}

	app.AppOptions = *opts

	if app.inheritor != nil {
		app.inheritor.(AppInheritorWhole).initAppInheritor(app)
	} else {
		app.inheritor = app
	}

	app.InitRunnable()
	app.Context = ctx

	CallOuter(app.autoRecover, app.GetReportError(), func() {
		if app.initFunc != nil {
			app.initFunc(app)
		}
	})
}

func (app *App) Run() chan struct{} {
	if !app.MarkRunning() {
		panic("app already running")
	}

	go func() {
		if parentCtx, ok := app.GetParentContext().(internal.Context); ok {
			parentCtx.GetWaitGroup().Add(1)
		}

		defer func() {
			if parentCtx, ok := app.GetParentContext().(internal.Context); ok {
				parentCtx.GetWaitGroup().Done()
			}

			app.GetWaitGroup().Wait()
			app.MarkShutdown()
			app.shutChan <- struct{}{}

			CallOuter(app.autoRecover, app.GetReportError(), func() {
				if app.stopFunc != nil {
					app.stopFunc(app)
				}
			})
		}()

		CallOuter(app.autoRecover, app.GetReportError(), func() {
			if app.startFunc != nil {
				app.startFunc(app)
			}
		})

		select {
		case <-app.Done():
			return
		}
	}()

	return app.shutChan
}

func (app *App) Stop() {
	app.GetCancelFunc()()
}

func (app *App) GetInheritor() internal.App {
	return app.inheritor
}

func (app *App) GetEntity(entID uint64) internal.Entity {
	entity, ok := app.entityMap.Load(entID)
	if !ok {
		return nil
	}

	return entity.(internal.Entity)
}

func (app *App) RangeEntities(fun func(entity internal.Entity) bool) {
	if fun == nil {
		return
	}

	app.entityMap.Range(func(key, value interface{}) bool {
		return fun(value.(internal.Entity))
	})
}

func (app *App) MakeUID() uint64 {
	return atomic.AddUint64(&app.uidMaker, 1)
}

func (app *App) AddEntity(entity internal.Entity) {
	if entity == nil {
		panic("nil entity")
	}

	if _, loaded := app.entityMap.LoadOrStore(entity.GetEntityID(), entity); loaded {
		panic("entity id already exists")
	}
}

func (app *App) RemoveEntity(entID uint64) {
	app.entityMap.Delete(entID)
}
