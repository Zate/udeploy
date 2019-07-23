package cache

import (
	"sync"

	"github.com/turnerlabs/udeploy/component/app"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Apps ...
var Apps appCache

func init() {
	Apps = appCache{
		apps:          map[string]app.Application{},
		lookup:        map[string]string{},
		Notifications: make(chan app.Application),
	}
}

type appCache struct {
	apps   map[string]app.Application
	lookup map[string]string
	mux    sync.Mutex

	Notifications chan app.Application
}

func (c *appCache) Update(app app.Application) {
	c.mux.Lock()
	defer c.mux.Unlock()

	for _, i := range app.Instances {
		c.lookup[i.Task.Definition.ID] = app.Name
	}

	if cachedApp, appFound := c.apps[app.Name]; appFound {
		for cachedName, cachedInst := range cachedApp.Instances {
			if instance, instFound := app.Instances[cachedName]; instFound {
				instance.RecordState(cachedInst.CurrentState)
				app.Instances[cachedName] = instance
			}
		}
	}

	c.apps[app.Name] = app

	c.Notifications <- app
}

func (c *appCache) UpdateInstances(appName string, instances map[string]app.Instance) {
	c.mux.Lock()
	defer c.mux.Unlock()

	if cachedApp, appFound := c.apps[appName]; appFound {
		for cachedName, cachedInst := range cachedApp.Instances {
			if instance, instFound := instances[cachedName]; instFound {
				instance.RecordState(cachedInst.CurrentState)
				instances[cachedName] = instance
			}
		}
	}

	for name, inst := range instances {
		c.apps[appName].Instances[name] = inst
	}

	c.Notifications <- c.apps[appName]
}

func (c *appCache) ResetChangeState(appName string, instName string) {
	c.mux.Lock()
	defer c.mux.Unlock()

	if cachedApp, appFound := c.apps[appName]; appFound {
		if instance, instFound := cachedApp.Instances[instName]; instFound {
			instance.RecordState(instance.CurrentState)
			cachedApp.Instances[instName] = instance
		}
	}
}

func (c *appCache) Remove(appName string) {
	c.mux.Lock()
	defer c.mux.Unlock()

	delete(c.apps, appName)
}

func (c *appCache) RemoveByID(appID primitive.ObjectID) {
	c.mux.Lock()
	defer c.mux.Unlock()

	for _, app := range c.apps {
		if appID == app.ID {
			delete(c.apps, app.Name)
		}
	}
}

func (c *appCache) GetAll() map[string]app.Application {
	c.mux.Lock()
	defer c.mux.Unlock()

	return c.apps
}

func (c *appCache) Get(appName string) (app.Application, bool) {
	c.mux.Lock()
	defer c.mux.Unlock()

	app, found := c.apps[appName]

	return app, found
}

func (c *appCache) GetByID(appID primitive.ObjectID) app.Application {
	c.mux.Lock()
	defer c.mux.Unlock()

	for _, app := range c.apps {
		if appID == app.ID {
			return app
		}
	}

	return app.Application{}
}

func (c *appCache) GetByDefinitionID(taskDefinition string) (app.Application, bool) {
	c.mux.Lock()
	defer c.mux.Unlock()

	appName, found := c.lookup[taskDefinition]
	if !found {
		return app.Application{}, false
	}

	a, found := c.apps[appName]
	if !found {
		return app.Application{}, false
	}

	return a, found
}

func (c *appCache) UpdateInstance(instance app.Instance) {
	c.mux.Lock()
	defer c.mux.Unlock()

	appName, found := c.lookup[instance.Task.Definition.ID]
	if !found {
		return
	}

	app, found := c.apps[appName]
	if !found {
		return
	}

	for name, i := range app.Instances {
		if i.Task.Definition.ID == instance.Task.Definition.ID {
			c.apps[appName].Instances[name] = instance
		}
	}

	return
}
