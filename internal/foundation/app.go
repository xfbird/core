package foundation

import (
	"github.com/pangdogs/core/internal"
	"sync"
	"sync/atomic"
)

type AppWhole interface {
	internal.App
	InitApp(ctx internal.Context)
	MakeUID() uint64
	AddEntity(entity internal.Entity)
	RemoveEntity(entID uint64)
	RangeEntities(func(entity internal.Entity) bool)
}

func NewApp(ctx internal.Context) internal.App {
	app := &App{}
	app.InitApp(ctx)
	return app
}

type App struct {
	Runnable
	internal.Context
	uidMaker  uint64
	entityMap sync.Map
}

func (app *App) InitApp(ctx internal.Context) {
	if ctx == nil {
		panic("nil ctx")
	}

	app.InitRunnable()
	app.Context = ctx
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
		}()

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

func (app *App) GetEntity(entID uint64) internal.Entity {
	entity, _ := app.entityMap.Load(entID)
	return entity.(internal.Entity)
}

func (app *App) MakeUID() uint64 {
	return atomic.AddUint64(&app.uidMaker, 1)
}

func (app *App) AddEntity(entity internal.Entity) {
	if entity == nil {
		panic("nil entity")
	}

	if _, loaded := app.entityMap.LoadOrStore(entity.GetEntityID(), entity.(EntityWhole).GetInheritor()); loaded {
		panic("entity id already exists")
	}
}

func (app *App) RemoveEntity(entID uint64) {
	app.entityMap.Delete(entID)
}

func (app *App) RangeEntities(fun func(entity internal.Entity) bool) {
	if fun == nil {
		return
	}

	app.entityMap.Range(func(key, value interface{}) bool {
		return fun(value.(internal.Entity))
	})
}