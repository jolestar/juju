// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package remoterelations_test

import (
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/api/base/testing"
	"github.com/juju/juju/api/remoterelations"
	"github.com/juju/juju/apiserver/params"
	coretesting "github.com/juju/juju/testing"
)

var _ = gc.Suite(&remoteRelationsSuite{})

type remoteRelationsSuite struct {
	coretesting.BaseSuite
}

func (s *remoteRelationsSuite) TestNewState(c *gc.C) {
	apiCaller := testing.APICallerFunc(func(objType string, version int, id, request string, arg, result interface{}) error {
		return nil
	})
	st := remoterelations.NewState(apiCaller)
	c.Assert(st, gc.NotNil)
}

func (s *remoteRelationsSuite) TestWatchRemoteApplications(c *gc.C) {
	var callCount int
	apiCaller := testing.APICallerFunc(func(objType string, version int, id, request string, arg, result interface{}) error {
		c.Check(objType, gc.Equals, "RemoteRelations")
		c.Check(version, gc.Equals, 0)
		c.Check(id, gc.Equals, "")
		c.Check(request, gc.Equals, "WatchRemoteApplications")
		c.Assert(result, gc.FitsTypeOf, &params.StringsWatchResult{})
		*(result.(*params.StringsWatchResult)) = params.StringsWatchResult{
			Error: &params.Error{Message: "FAIL"},
		}
		callCount++
		return nil
	})
	st := remoterelations.NewState(apiCaller)
	_, err := st.WatchRemoteApplications()
	c.Check(err, gc.ErrorMatches, "FAIL")
	c.Check(callCount, gc.Equals, 1)
}

func (s *remoteRelationsSuite) TestWatchRemoteApplication(c *gc.C) {
	var callCount int
	apiCaller := testing.APICallerFunc(func(objType string, version int, id, request string, arg, result interface{}) error {
		c.Check(objType, gc.Equals, "RemoteRelations")
		c.Check(version, gc.Equals, 0)
		c.Check(id, gc.Equals, "")
		c.Check(request, gc.Equals, "WatchRemoteApplication")
		c.Assert(result, gc.FitsTypeOf, &params.ApplicationRelationsWatchResults{})
		*(result.(*params.ApplicationRelationsWatchResults)) = params.ApplicationRelationsWatchResults{
			Results: []params.ApplicationRelationsWatchResult{{
				Error: &params.Error{Message: "FAIL"},
			}},
		}
		callCount++
		return nil
	})
	st := remoterelations.NewState(apiCaller)
	_, err := st.WatchRemoteApplication("db2")
	c.Check(err, gc.ErrorMatches, "FAIL")
	c.Check(callCount, gc.Equals, 1)
}

func (s *remoteRelationsSuite) TestWatchRemoteApplicationInvalidService(c *gc.C) {
	apiCaller := testing.APICallerFunc(func(objType string, version int, id, request string, arg, result interface{}) error {
		return nil
	})
	st := remoterelations.NewState(apiCaller)
	_, err := st.WatchRemoteApplication("!@#")
	c.Assert(err, gc.ErrorMatches, `application name "!@#" not valid`)
}