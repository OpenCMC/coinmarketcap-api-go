package coinmarketcap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
)

// TestEveryMethodCallsItsEndpoint calls each generated method with empty
// params and checks the HTTP method and path the server receives.
func TestEveryMethodCallsItsEndpoint(t *testing.T) {
	var mu sync.Mutex
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotMethod, gotPath = r.Method, r.URL.Path
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	cmc := mustNew(t, WithAPIKey("key"), WithBaseURL(srv.URL), WithMaxRetries(0))

	if len(operationIndex) == 0 {
		t.Fatal("operationIndex is empty")
	}
	services := reflect.ValueOf(cmc.Services)
	covered := map[string]bool{}
	for _, op := range operationIndex {
		name := op.Service + "." + op.Method
		if covered[name] {
			t.Errorf("%s is generated twice", name)
		}
		covered[op.Service] = true
		covered[name] = true

		svc := services.FieldByName(op.Service)
		if !svc.IsValid() || svc.IsNil() {
			t.Errorf("no service %s", op.Service)
			continue
		}
		method := svc.MethodByName(op.Method)
		if !method.IsValid() {
			t.Errorf("no method %s", name)
			continue
		}
		mt := method.Type()
		args := []reflect.Value{reflect.ValueOf(context.Background())}
		for i := 1; i < mt.NumIn(); i++ {
			if mt.IsVariadic() && i == mt.NumIn()-1 {
				break
			}
			args = append(args, reflect.Zero(mt.In(i)))
		}
		out := method.Call(args)
		if err, _ := out[1].Interface().(error); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		mu.Lock()
		if gotMethod != op.HTTPMethod || gotPath != op.Path {
			t.Errorf("%s sent %s %s, want %s %s", name, gotMethod, gotPath, op.HTTPMethod, op.Path)
		}
		mu.Unlock()
	}
	for i := 0; i < services.NumField(); i++ {
		if field := services.Type().Field(i).Name; !covered[field] {
			t.Errorf("service %s has no operations in operationIndex", field)
		}
	}
}
