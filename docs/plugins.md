# Beskar Plugins
A Beskar plugin is a binary that is deployed alongside Beskar and is responsible for managing a specific type of
artifact. For example, the `yum` plugin is responsible for managing RPMs. In it's very basic form a plugin is responsible
for mapping an incoming request to an artifact in the registry. Plugins may contain additional logic to support other actions
such as uploading, deleting, etc.  For example, the `yum` plugin supports mirroring a remote repository.

## How To Use This Document
This document is intended to be a guide for writing a Beskar plugin. It is not intended to be a complete reference. Use 
the information provided here to get started and then refer to the code for more details. `internal/plugins/static` is a
simple plugin to use as a reference. It is recommended that you read through the code and then use it as a starting point
for your own plugin.

## Plugin Architecture
A Beskar plugin is written in Go and will be deployed so that it can be accessed by Beskar. There are a few mechanisms that 
Beskar uses to discover and communicate with plugins. The first of which is a gossip protocol that is used to discover
plugins. The second is the Events API that is used to keep plugins in sync with Beskar, such as when an artifact is uploaded 
or deleted. The third is the plugin service that is used to serve the plugin's API.  We will cover these in more detail below,
but luckily Beskar provides a few interfaces, as well as a series of helper methods, to make writing a plugin easier.

### Plugin Discovery and API Request Routing
Beskar uses [a gossip protocol](https://github.com/hashicorp/memberlist) to discover plugins. Early in its startup process a plugin will register itself 
with a known peer, generally one of the main Beskar instances, and the plugin's info will be shared with the rest of the cluster.
This info includes the plugin's name, version, and the address of the plugin's API. Beskar will then use this info to route
requests to the plugin's API using a [Rego policy](https://www.openpolicyagent.org/) provided by the plugin. 

**Note that you do not need to do anything special to register your plugin. Beskar will handle this for you.** All you need
to do is provide the plugin's info, which includes the rego policy, and a router. We will cover this in more detail later.

### Repository Handler
In some cases your plugin may need to be informed when an artifact is uploaded or deleted. This is accomplished by
implementing the [Handler interface](../internal/pkg/repository/handler.go). The object you implement will be used to receive events from Beskar and will 
enable your plugin to keep its internal state up to date.

#### Implementation Notes
When implementing your `repository.Handler` there are a few things to keep in mind.

First, the `QueueEvent()` method is not intended to be used to perform long-running operations. Instead, you should 
queue the event for processing in another goroutine. The static plugin provides a good example of this by spinning
up a goroutine in its `Start()` that listens for events and processes them, while the `QueueEvent()` method simply queues
the event for processing in the run loop.

Second, Beskar provides a [RepoHandler struct](../internal/pkg/repository/handler.go) that partially implements the 
`Handler` interface and provides some helper methods that reduce your implementation burden to only `Start()` and 
`QueueEvent()`. This is exemplified below as well as in the [Static plugin](../internal/plugins/static/pkg/staticrepository/handler.go).

Third, we recommend that you create a constructor for your handler that conforms to the `repository.HandlerFactory` type. 
This will come in handy later when creating the plugin service.

#### Example Implementation of `repository.Handler`
```

type ExampleHandler struct {
    *repository.RepoHandler
}
 
func NewExampleHandler(*slog.Logger, repoHandler *repository.RepoHandler) *ExampleHandler {
    return &ExampleHandler{
        RepoHandler: repoHandler,
    }
}

func (h *ExampleHandler) Start(ctx context.Context) {
    // Process stored events
    // Start goroutine to dequeue and process new events
}

func (h *ExampleHandler) QueueEvent(event *eventv1.EventPayload, store bool) error {
    // Store event if store is true
    // Queue event for processing
    return nil
}
```

#### Plugins without internal state
Not all plugins will have internal state, for example, the [Static plugin](../internal/plugins/ostree/plugin.go). simply
maps an incoming request to an artifact in the registry. In these cases, it is not required to implement a 
`repository.Handler`. You can simply return `nil` from the `RepositoryManager()` method of your plugin service and leave
your plugin's `Info.MediaTypes` empty. This will tell Beskar that your plugin does not need to receive events. More on 
this in the next section.


### Plugin Service
The [Plugin Service](../internal/pkg/pluginsrv/service.go) is responsible for serving the plugin's API, registering your
`repository.Handler` and providing the info Beskar needs about your plugin. We recommend that your implementation of 
`pluginsrv.Service` have a constructor that accepts a config object and returns a pointer to your service. For example:
```

//go:embed embedded/router.rego
var routerRego []byte

//go:embed embedded/data.json
var routerData []byte

const (
    // PluginName is the name of the plugin
    PluginName = "example"
)

type ExamplePlugin struct {
    ctx context.Context
    config pluginsrv.Config
    
    repositoryManager *repository.Manager
    handlerParams     *repository.HandlerParams
}

type ExamplePluginConfig struct {
    Gossip gossip.Config
}

func NewExamplePlugin(ctx context.Context, exampleConfig ExamplePluginConfig) (*ExamplePlugin, error) {
    config := pluginsrv.Config{}
    
    router := chi.NewRouter()
    // for kubernetes probes
    router.Handle("/", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	config.Router = router
	config.Gossip = exampleConfig.Gossip
	config.Info = &pluginv1.Info{
		Name:       PluginName,
		Version:    version.Semver,
		Mediatypes: []string{
		    "application/vnd.ciq.example.file.v1.config+json",
		},
		Router: &pluginv1.Router{
			Rego: routerRego,
			Data: routerData,
		},
	}

    plugin := ExamplePlugin{
        ctx: ctx,
        config: config,
    }
    
    plugin.repositoryManager = repository.NewManager(plugin.handlerParams, NewExampleHandler)
     
    return &plugin, nil
}

func (p *ExamplePlugin) Start(http.RoundTripper, *mtls.CAPEM, *gossip.BeskarMeta) error {
    // Register handlers with p.config.Router
    return nil
}

func (p *ExamplePlugin) Context() context.Context {
    return p.ctx
}

func (p *ExamplePlugin) Config() Config {
    return p.config
}

func (p *ExamplePlugin) RepositoryManager() *repository.Manager {
    return nil
}
```


#### Your Plugin's API
The `Start(...)` method is called when the server is about to serve your plugin's api and is your chance to register your
plugin's handlers with the server.

The `Config()` method is used to return your plugin's configuration. This is used by Beskar to generate the plugin's

