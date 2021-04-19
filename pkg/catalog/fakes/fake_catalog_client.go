// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"

	"github.com/weaveworks/pctl/pkg/catalog"
)

type FakeCatalogClient struct {
	DoRequestStub        func(string, map[string]string) ([]byte, error)
	doRequestMutex       sync.RWMutex
	doRequestArgsForCall []struct {
		arg1 string
		arg2 map[string]string
	}
	doRequestReturns struct {
		result1 []byte
		result2 error
	}
	doRequestReturnsOnCall map[int]struct {
		result1 []byte
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeCatalogClient) DoRequest(arg1 string, arg2 map[string]string) ([]byte, error) {
	fake.doRequestMutex.Lock()
	ret, specificReturn := fake.doRequestReturnsOnCall[len(fake.doRequestArgsForCall)]
	fake.doRequestArgsForCall = append(fake.doRequestArgsForCall, struct {
		arg1 string
		arg2 map[string]string
	}{arg1, arg2})
	stub := fake.DoRequestStub
	fakeReturns := fake.doRequestReturns
	fake.recordInvocation("DoRequest", []interface{}{arg1, arg2})
	fake.doRequestMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeCatalogClient) DoRequestCallCount() int {
	fake.doRequestMutex.RLock()
	defer fake.doRequestMutex.RUnlock()
	return len(fake.doRequestArgsForCall)
}

func (fake *FakeCatalogClient) DoRequestCalls(stub func(string, map[string]string) ([]byte, error)) {
	fake.doRequestMutex.Lock()
	defer fake.doRequestMutex.Unlock()
	fake.DoRequestStub = stub
}

func (fake *FakeCatalogClient) DoRequestArgsForCall(i int) (string, map[string]string) {
	fake.doRequestMutex.RLock()
	defer fake.doRequestMutex.RUnlock()
	argsForCall := fake.doRequestArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeCatalogClient) DoRequestReturns(result1 []byte, result2 error) {
	fake.doRequestMutex.Lock()
	defer fake.doRequestMutex.Unlock()
	fake.DoRequestStub = nil
	fake.doRequestReturns = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

func (fake *FakeCatalogClient) DoRequestReturnsOnCall(i int, result1 []byte, result2 error) {
	fake.doRequestMutex.Lock()
	defer fake.doRequestMutex.Unlock()
	fake.DoRequestStub = nil
	if fake.doRequestReturnsOnCall == nil {
		fake.doRequestReturnsOnCall = make(map[int]struct {
			result1 []byte
			result2 error
		})
	}
	fake.doRequestReturnsOnCall[i] = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

func (fake *FakeCatalogClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.doRequestMutex.RLock()
	defer fake.doRequestMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeCatalogClient) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ catalog.CatalogClient = new(FakeCatalogClient)
