// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package model_test

import (
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/testing"
)

type fakeEnvSuite struct {
	testing.FakeJujuXDGDataHomeSuite
	fake *fakeEnvAPI
}

func (s *fakeEnvSuite) SetUpTest(c *gc.C) {
	s.FakeJujuXDGDataHomeSuite.SetUpTest(c)
	s.fake = &fakeEnvAPI{
		values: map[string]interface{}{
			"name":    "test-model",
			"special": "special value",
			"running": true,
		},
		defaults: config.ConfigValues{
			"attr":  {Value: "foo", Source: "default"},
			"attr2": {Value: "bar", Source: "controller"},
		},
	}
}

type fakeEnvAPI struct {
	values   map[string]interface{}
	defaults config.ConfigValues
	err      error
	keys     []string
}

func (f *fakeEnvAPI) Close() error {
	return nil
}

func (f *fakeEnvAPI) ModelGet() (map[string]interface{}, error) {
	return f.values, nil
}

func (f *fakeEnvAPI) ModelGetWithMetadata() (config.ConfigValues, error) {
	result := make(config.ConfigValues)
	for name, val := range f.values {
		result[name] = config.ConfigValue{Value: val, Source: "model"}
	}
	return result, nil
}

func (f *fakeEnvAPI) ModelDefaults() (config.ConfigValues, error) {
	return f.defaults, nil
}

func (f *fakeEnvAPI) SetModelDefaults(cfg map[string]interface{}) error {
	if f.err != nil {
		return f.err
	}
	for name, val := range cfg {
		f.defaults[name] = config.ConfigValue{Value: val, Source: "controller"}
	}
	return nil
}

func (f *fakeEnvAPI) UnsetModelDefaults(keys ...string) error {
	if f.err != nil {
		return f.err
	}
	for _, key := range keys {
		delete(f.defaults, key)
	}
	return nil
}

func (f *fakeEnvAPI) ModelSet(config map[string]interface{}) error {
	f.values = config
	return f.err
}

func (f *fakeEnvAPI) ModelUnset(keys ...string) error {
	f.keys = keys
	return f.err
}
