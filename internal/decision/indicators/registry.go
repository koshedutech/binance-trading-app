// Package indicators provides an extensible indicator framework for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.12: Indicator Segment Framework
package indicators

import (
	"fmt"
	"sort"
	"sync"
)

// IndicatorRegistry manages all available indicators.
// Thread-safe for concurrent access.
type IndicatorRegistry struct {
	mu         sync.RWMutex
	indicators map[string]Indicator
	bySegment  map[IndicatorSegment][]string // segment -> indicator names
	info       map[string]*IndicatorInfo     // metadata for each indicator
}

// NewIndicatorRegistry creates a new empty registry.
func NewIndicatorRegistry() *IndicatorRegistry {
	return &IndicatorRegistry{
		indicators: make(map[string]Indicator),
		bySegment:  make(map[IndicatorSegment][]string),
		info:       make(map[string]*IndicatorInfo),
	}
}

// Register adds an indicator to the registry.
// Returns an error if an indicator with the same name already exists.
func (r *IndicatorRegistry) Register(ind Indicator) error {
	if ind == nil {
		return fmt.Errorf("cannot register nil indicator")
	}

	name := ind.Name()
	if name == "" {
		return fmt.Errorf("indicator name cannot be empty")
	}

	segment := ind.Segment()
	if !segment.IsValid() {
		return fmt.Errorf("invalid indicator segment: %s", segment)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate
	if _, exists := r.indicators[name]; exists {
		return fmt.Errorf("indicator already registered: %s", name)
	}

	// Register the indicator
	r.indicators[name] = ind

	// Add to segment index
	if r.bySegment[segment] == nil {
		r.bySegment[segment] = []string{}
	}
	r.bySegment[segment] = append(r.bySegment[segment], name)

	// Store metadata
	r.info[name] = NewIndicatorInfo(ind)

	return nil
}

// Unregister removes an indicator from the registry.
// Returns an error if the indicator doesn't exist.
func (r *IndicatorRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ind, exists := r.indicators[name]
	if !exists {
		return fmt.Errorf("indicator not found: %s", name)
	}

	// Remove from segment index
	segment := ind.Segment()
	if names, ok := r.bySegment[segment]; ok {
		filtered := make([]string, 0, len(names)-1)
		for _, n := range names {
			if n != name {
				filtered = append(filtered, n)
			}
		}
		r.bySegment[segment] = filtered
	}

	// Remove from maps
	delete(r.indicators, name)
	delete(r.info, name)

	return nil
}

// Get retrieves an indicator by name.
// Returns the indicator and true if found, nil and false otherwise.
func (r *IndicatorRegistry) Get(name string) (Indicator, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ind, exists := r.indicators[name]
	return ind, exists
}

// MustGet retrieves an indicator by name and panics if not found.
// Use only when you're certain the indicator exists.
func (r *IndicatorRegistry) MustGet(name string) Indicator {
	ind, exists := r.Get(name)
	if !exists {
		panic(fmt.Sprintf("indicator not found: %s", name))
	}
	return ind
}

// GetBySegment returns all indicators for a given segment.
// Returns an empty slice if no indicators are registered for the segment.
func (r *IndicatorRegistry) GetBySegment(segment IndicatorSegment) []Indicator {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names, ok := r.bySegment[segment]
	if !ok || len(names) == 0 {
		return []Indicator{}
	}

	result := make([]Indicator, 0, len(names))
	for _, name := range names {
		if ind, exists := r.indicators[name]; exists {
			result = append(result, ind)
		}
	}

	return result
}

// GetNamesBySegment returns the names of all indicators for a given segment.
func (r *IndicatorRegistry) GetNamesBySegment(segment IndicatorSegment) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names, ok := r.bySegment[segment]
	if !ok {
		return []string{}
	}

	// Return a copy to prevent modification
	result := make([]string, len(names))
	copy(result, names)
	return result
}

// ListAll returns all registered indicator names, sorted alphabetically.
func (r *IndicatorRegistry) ListAll() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.indicators))
	for name := range r.indicators {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Count returns the total number of registered indicators.
func (r *IndicatorRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.indicators)
}

// CountBySegment returns the number of indicators per segment.
func (r *IndicatorRegistry) CountBySegment() map[IndicatorSegment]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[IndicatorSegment]int)
	for segment, names := range r.bySegment {
		result[segment] = len(names)
	}
	return result
}

// Validate checks if all the specified indicator names exist in the registry.
// Returns an error listing all missing indicators.
func (r *IndicatorRegistry) Validate(names []string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var missing []string
	for _, name := range names {
		if _, exists := r.indicators[name]; !exists {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("indicators not found: %v", missing)
	}
	return nil
}

// ValidateForSegment checks if all specified indicators exist and belong to the given segment.
func (r *IndicatorRegistry) ValidateForSegment(names []string, segment IndicatorSegment) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var missing []string
	var wrongSegment []string

	for _, name := range names {
		ind, exists := r.indicators[name]
		if !exists {
			missing = append(missing, name)
			continue
		}
		if ind.Segment() != segment {
			wrongSegment = append(wrongSegment, fmt.Sprintf("%s (is %s, expected %s)", name, ind.Segment(), segment))
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("indicators not found: %v", missing)
	}
	if len(wrongSegment) > 0 {
		return fmt.Errorf("indicators in wrong segment: %v", wrongSegment)
	}
	return nil
}

// GetInfo returns the metadata for a registered indicator.
func (r *IndicatorRegistry) GetInfo(name string) (*IndicatorInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.info[name]
	if !exists {
		return nil, false
	}

	// Return a copy
	infoCopy := *info
	if info.Dependencies != nil {
		infoCopy.Dependencies = make([]string, len(info.Dependencies))
		copy(infoCopy.Dependencies, info.Dependencies)
	}
	return &infoCopy, true
}

// GetAllInfo returns metadata for all registered indicators.
func (r *IndicatorRegistry) GetAllInfo() []*IndicatorInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*IndicatorInfo, 0, len(r.info))
	for _, info := range r.info {
		infoCopy := *info
		if info.Dependencies != nil {
			infoCopy.Dependencies = make([]string, len(info.Dependencies))
			copy(infoCopy.Dependencies, info.Dependencies)
		}
		result = append(result, &infoCopy)
	}
	return result
}

// SetEnabled enables or disables an indicator without removing it.
func (r *IndicatorRegistry) SetEnabled(name string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info, exists := r.info[name]
	if !exists {
		return fmt.Errorf("indicator not found: %s", name)
	}
	info.Enabled = enabled
	return nil
}

// IsEnabled checks if an indicator is enabled.
func (r *IndicatorRegistry) IsEnabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.info[name]
	if !exists {
		return false
	}
	return info.Enabled
}

// GetEnabledBySegment returns only enabled indicators for a segment.
func (r *IndicatorRegistry) GetEnabledBySegment(segment IndicatorSegment) []Indicator {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names, ok := r.bySegment[segment]
	if !ok || len(names) == 0 {
		return []Indicator{}
	}

	result := make([]Indicator, 0, len(names))
	for _, name := range names {
		if info, exists := r.info[name]; exists && info.Enabled {
			if ind, ok := r.indicators[name]; ok {
				result = append(result, ind)
			}
		}
	}

	return result
}

// ResolveDependencies returns indicators in dependency order (dependencies first).
// Uses topological sort to ensure correct calculation order.
func (r *IndicatorRegistry) ResolveDependencies(names []string) ([]Indicator, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Build dependency graph
	visited := make(map[string]bool)
	inProgress := make(map[string]bool)
	result := make([]Indicator, 0, len(names))

	var visit func(name string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		if inProgress[name] {
			return fmt.Errorf("circular dependency detected involving: %s", name)
		}

		ind, exists := r.indicators[name]
		if !exists {
			return fmt.Errorf("indicator not found: %s", name)
		}

		inProgress[name] = true

		// Visit dependencies first
		for _, dep := range ind.Dependencies() {
			if err := visit(dep); err != nil {
				return err
			}
		}

		inProgress[name] = false
		visited[name] = true
		result = append(result, ind)
		return nil
	}

	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// Global registry pattern for convenience

var globalIndicatorRegistry *IndicatorRegistry
var indicatorRegistryOnce sync.Once

// GetGlobalIndicatorRegistry returns the global indicator registry singleton.
func GetGlobalIndicatorRegistry() *IndicatorRegistry {
	indicatorRegistryOnce.Do(func() {
		globalIndicatorRegistry = NewIndicatorRegistry()
	})
	return globalIndicatorRegistry
}

// RegisterGlobalIndicator registers an indicator with the global registry.
func RegisterGlobalIndicator(ind Indicator) error {
	return GetGlobalIndicatorRegistry().Register(ind)
}

// GetGlobalIndicator retrieves an indicator from the global registry.
func GetGlobalIndicator(name string) (Indicator, bool) {
	return GetGlobalIndicatorRegistry().Get(name)
}

// ValidateGlobalIndicators validates indicator names against the global registry.
func ValidateGlobalIndicators(names []string) error {
	return GetGlobalIndicatorRegistry().Validate(names)
}
