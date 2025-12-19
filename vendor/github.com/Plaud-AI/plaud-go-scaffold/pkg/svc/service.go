package svc

import (
	"context"
	"reflect"
	"slices"

	"github.com/Plaud-AI/plaud-go-scaffold/pkg/logger"
)

type Initable interface {
	Init(ctx context.Context) error
}

type Startable interface {
	Start(ctx context.Context) error
}

type Stoppable interface {
	Stop(ctx context.Context) error
}

// Service defines the minimal lifecycle that concrete services should implement.
type Service interface {
	Initable
	Startable
	Stoppable
}

type BaseService struct {
	inited  bool
	started bool
	stopped bool
}

func (s *BaseService) IsInited() bool {
	return s.inited
}

func (s *BaseService) IsStarted() bool {
	return s.started
}

func (s *BaseService) IsStopped() bool {
	return s.stopped
}

func (s *BaseService) SetInited(inited bool) {
	s.inited = inited
}

func (s *BaseService) SetStarted(started bool) {
	s.started = started
}

func (s *BaseService) SetStopped(stopped bool) {
	s.stopped = stopped
}

// InitAll scans container's exported fields (struct or *struct) and calls Init on any match.
func InitAll(ctx context.Context, container interface{}) error {
	logger.Infof("init all services for %T", container)
	for _, s := range collectValuesOf[Initable](container) {
		logger.Infof("init service: %T", s)
		if err := s.Init(ctx); err != nil {
			logger.Errorf("init service %T error: %v", s, err)
			return err
		}
		logger.Infof("init service %T success", s)
	}
	return nil
}

// StartAll scans container's exported fields (struct or *struct) and calls Start on any match.
func StartAll(ctx context.Context, container interface{}) error {
	logger.Infof("start all services for %T", container)
	for _, s := range collectValuesOf[Startable](container) {
		logger.Infof("start service: %T", s)
		if err := s.Start(ctx); err != nil {
			logger.Errorf("start service %T error: %v", s, err)
			return err
		}
		logger.Infof("start service %T success", s)
	}
	return nil
}

// StopAll scans container's exported fields (struct or *struct) and calls Stop on any match.
func StopAll(ctx context.Context, container interface{}) error {
	logger.Infof("stop all services for %T", container)
	stoppableServices := collectValuesOf[Stoppable](container)
	slices.Reverse(stoppableServices)
	for _, s := range stoppableServices {
		logger.Infof("stop service: %T", s)
		if err := s.Stop(ctx); err != nil {
			logger.Errorf("stop service %T error: %v", s, err)
			return err
		}
		logger.Infof("stop service %T success", s)
	}
	return nil
}

// collectValuesOf returns exported values from a container (struct or *struct),
// including nested fields (embedded structs, pointers to structs, and interfaces)
// that implement interface T. Nil pointers/interfaces are skipped.
func collectValuesOf[T any](container interface{}) []T {
	var out []T
	visited := map[uintptr]struct{}{}

	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		// unwrap pointers and prevent cycles
		for v.Kind() == reflect.Ptr {
			if v.IsNil() {
				return
			}
			ptr := v.Pointer()
			if _, ok := visited[ptr]; ok {
				return
			}
			visited[ptr] = struct{}{}
			v = v.Elem()
		}

		// collect the value itself if it implements T
		if v.CanInterface() {
			if s, ok := v.Interface().(T); ok {
				out = append(out, s)
			}
		}

		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				f := v.Field(i)
				if !f.CanInterface() {
					continue
				}
				// skip nil pointers/interfaces
				if (f.Kind() == reflect.Ptr || f.Kind() == reflect.Interface) && f.IsNil() {
					continue
				}
				// try collect direct field
				if s, ok := f.Interface().(T); ok {
					out = append(out, s)
				}
				// only recurse into embedded (anonymous) fields
				fieldInfo := v.Type().Field(i)
				if fieldInfo.Anonymous {
					switch f.Kind() {
					case reflect.Struct, reflect.Ptr, reflect.Interface:
						walk(f)
					}
				}
			}
		case reflect.Interface:
			if !v.IsNil() {
				walk(v.Elem())
			}
		}
	}

	walk(reflect.ValueOf(container))
	return out
}
