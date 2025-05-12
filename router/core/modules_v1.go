package core

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"context"
	"go.uber.org/zap"
)

type moduleRegistry struct {
	mu sync.RWMutex
	modules map[string]MyModuleInfo
}
// NewModuleRegistry returns an empty, thread-safe module registry.
// Call this in tests (and anywhere you need isolation) instead of using the global.
func newModuleRegistry() *moduleRegistry {
	return &moduleRegistry{
		modules: make(map[string]MyModuleInfo),
	}
}

// TODO: @kaialang discuss if we should push for dependency injection.
// defaultModuleRegistry is the package-level registry used by RegisterMyModule.
// For unit tests you should use newModuleRegistry() to get a fresh instance and avoid shared state.
var defaultModuleRegistry = newModuleRegistry()


type MyModuleInfo struct {
	// ID is the unique identifier for a module, it must be unique across all modules.
	ID string
	// Priority decideds the order of execution of the module.
	// The smaller the number, the higher the priority, the earlier the module is executed.
	// For example, a priority of 0 is the highest priority.
	// Modules with the same priority are executed in the order they are registered.
	// If Priority is nil, the module is considered to have the lowest priority.
	Priority *int
	// New creates a new instance of the module.
	New func() MyModule
}

type MyModule interface {
	MyModule() MyModuleInfo
}

// RegisterMyModule registers a new MyModule instance.
// The registration order matters. Modules with the same priority 
// are executed in the order they are registered.
// It panics if the module is already registered.
func RegisterMyModule(instance MyModule) {
	defaultModuleRegistry.registerMyModule(instance)
}

func (r *moduleRegistry) registerMyModule(instance MyModule) {
	m := instance.MyModule()

	if m.ID == "" {
		panic("MyModule.ID is required")
	}
	if val := m.New(); val == nil {
		panic("MyModuleInfo.New must return a non-nil module instance")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.modules[m.ID]; ok {
		panic(fmt.Sprintf("MyModule already registered: %s", m.ID))
	}
	r.modules[m.ID] = m
}

// sortMyModules sorts the modules by priority, 0 is the highest priority, is the first to be executed.
// If two modules have the same priority, they are sorted by registration order.
// If a module has no priority, it is considered to have the lowest priority.
func sortMyModules(modules []MyModuleInfo) []MyModuleInfo {
	sort.Slice(modules, func(i, j int) bool {
		var priorityI, priorityJ int = math.MaxInt, math.MaxInt
		if modules[i].Priority != nil {
			priorityI = *modules[i].Priority
		}
		if modules[j].Priority != nil {
			priorityJ = *modules[j].Priority
		}

		return priorityI < priorityJ
	})
	return modules
}

// getMyModules returns all registered modules sorted by priority
func (r *moduleRegistry) getMyModules() []MyModuleInfo {
    r.mu.RLock()
    defer r.mu.RUnlock()

    modules := make([]MyModuleInfo, 0, len(r.modules))
    for _, m := range r.modules {
        modules = append(modules, m)
    }
    return sortMyModules(modules)
}

// coreModules manages module initialization and hook registration.
type coreModules struct {
	hookRegistry *hookRegistry
	logger *zap.Logger
}

// newCoreModules initializes with an empty registry.
func newCoreModules(logger *zap.Logger) *coreModules {
	return &coreModules{
		hookRegistry: newHookRegistry(),
		logger: logger,
	}
}

// initMyModules instantiates each module, registers any implemented hooks, and saves the hook registry.
func (c *coreModules) initMyModules(ctx context.Context, modules []MyModuleInfo) error {
	hookRegistry := newHookRegistry()

	for _, info := range modules {
		moduleInstance := info.New()

		hookRegistry.AddApplicationLifecycle(moduleInstance)
		hookRegistry.AddGraphQLServerLifecycle(moduleInstance)
		hookRegistry.AddRouterLifecycle(moduleInstance)
		hookRegistry.AddSubgraphLifecycle(moduleInstance)
		hookRegistry.AddOperationLifecycle(moduleInstance)
	}

	c.hookRegistry = hookRegistry

	return nil
}
