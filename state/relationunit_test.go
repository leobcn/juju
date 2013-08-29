// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state_test

import (
	"sort"
	"time"

	gc "launchpad.net/gocheck"

	"launchpad.net/juju-core/charm"
	"launchpad.net/juju-core/errors"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/state/testing"
	coretesting "launchpad.net/juju-core/testing"
	"launchpad.net/juju-core/testing/checkers"
)

type RelationUnitSuite struct {
	ConnSuite
}

var _ = gc.Suite(&RelationUnitSuite{})

func (s *RelationUnitSuite) assertInScope(c *gc.C, ru *state.RelationUnit, inScope bool) {
	ok, err := ru.InScope()
	c.Assert(err, gc.IsNil)
	c.Assert(ok, gc.Equals, inScope)
}

func (s *RelationUnitSuite) TestReadSettingsErrors(c *gc.C) {
	riak, err := s.State.AddService("riak", s.AddTestingCharm(c, "riak"))
	c.Assert(err, gc.IsNil)
	u0, err := riak.AddUnit()
	c.Assert(err, gc.IsNil)
	riakEP, err := riak.Endpoint("ring")
	c.Assert(err, gc.IsNil)
	rel, err := s.State.EndpointsRelation(riakEP)
	c.Assert(err, gc.IsNil)
	ru0, err := rel.Unit(u0)
	c.Assert(err, gc.IsNil)

	_, err = ru0.ReadSettings("nonsense")
	c.Assert(err, gc.ErrorMatches, `cannot read settings for unit "nonsense" in relation "riak:ring": "nonsense" is not a valid unit name`)
	_, err = ru0.ReadSettings("unknown/0")
	c.Assert(err, gc.ErrorMatches, `cannot read settings for unit "unknown/0" in relation "riak:ring": service "unknown" is not a member of "riak:ring"`)
	_, err = ru0.ReadSettings("riak/pressure")
	c.Assert(err, gc.ErrorMatches, `cannot read settings for unit "riak/pressure" in relation "riak:ring": "riak/pressure" is not a valid unit name`)
	_, err = ru0.ReadSettings("riak/1")
	c.Assert(err, gc.ErrorMatches, `cannot read settings for unit "riak/1" in relation "riak:ring": settings not found`)
}

func (s *RelationUnitSuite) TestPeerSettings(c *gc.C) {
	pr := NewPeerRelation(c, s.State)
	rus := RUs{pr.ru0, pr.ru1}

	// Check missing settings cannot be read by any RU.
	for _, ru := range rus {
		_, err := ru.ReadSettings("riak/0")
		c.Assert(err, gc.ErrorMatches, `cannot read settings for unit "riak/0" in relation "riak:ring": settings not found`)
	}

	// Add settings for one RU.
	s.assertInScope(c, pr.ru0, false)
	err := pr.ru0.EnterScope(map[string]interface{}{"gene": "kelly"})
	c.Assert(err, gc.IsNil)
	node, err := pr.ru0.Settings()
	c.Assert(err, gc.IsNil)
	node.Set("meme", "socially-awkward-penguin")
	_, err = node.Write()
	c.Assert(err, gc.IsNil)
	normal := map[string]interface{}{
		"gene": "kelly",
		"meme": "socially-awkward-penguin",
	}

	// Check settings can be read by every RU.
	assertSettings := func(u *state.Unit, expect map[string]interface{}) {
		for _, ru := range rus {
			m, err := ru.ReadSettings(u.Name())
			c.Assert(err, gc.IsNil)
			c.Assert(m, gc.DeepEquals, expect)
		}
	}
	assertSettings(pr.u0, normal)
	s.assertInScope(c, pr.ru0, true)

	// Check that EnterScope when scope already entered does not touch
	// settings at all.
	changed := map[string]interface{}{"foo": "bar"}
	err = pr.ru0.EnterScope(changed)
	c.Assert(err, gc.IsNil)
	assertSettings(pr.u0, normal)
	s.assertInScope(c, pr.ru0, true)

	// Leave scope, check settings are still as accessible as before.
	err = pr.ru0.LeaveScope()
	c.Assert(err, gc.IsNil)
	assertSettings(pr.u0, normal)
	s.assertInScope(c, pr.ru0, false)

	// Re-enter scope wih changed settings, and check they completely overwrite
	// the old ones.
	err = pr.ru0.EnterScope(changed)
	c.Assert(err, gc.IsNil)
	assertSettings(pr.u0, changed)
	s.assertInScope(c, pr.ru0, true)

	// Leave and re-enter with nil nettings, and check they overwrite to become
	// an empty map.
	err = pr.ru0.LeaveScope()
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, pr.ru0, false)
	err = pr.ru0.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	assertSettings(pr.u0, map[string]interface{}{})
	s.assertInScope(c, pr.ru0, true)

	// Check that entering scope for the first time with nil settings works correctly.
	s.assertInScope(c, pr.ru1, false)
	err = pr.ru1.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	assertSettings(pr.u1, map[string]interface{}{})
	s.assertInScope(c, pr.ru1, true)
}

func (s *RelationUnitSuite) TestProReqSettings(c *gc.C) {
	prr := NewProReqRelation(c, &s.ConnSuite, charm.ScopeGlobal)
	rus := RUs{prr.pru0, prr.pru1, prr.rru0, prr.rru1}

	// Check missing settings cannot be read by any RU.
	for _, ru := range rus {
		_, err := ru.ReadSettings("mysql/0")
		c.Assert(err, gc.ErrorMatches, `cannot read settings for unit "mysql/0" in relation "wordpress:db mysql:server": settings not found`)
	}

	// Add settings for one RU.
	s.assertInScope(c, prr.pru0, false)
	err := prr.pru0.EnterScope(map[string]interface{}{"gene": "simmons"})
	c.Assert(err, gc.IsNil)
	node, err := prr.pru0.Settings()
	c.Assert(err, gc.IsNil)
	node.Set("meme", "foul-bachelor-frog")
	_, err = node.Write()
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, prr.pru0, true)

	// Check settings can be read by every RU.
	for _, ru := range rus {
		m, err := ru.ReadSettings("mysql/0")
		c.Assert(err, gc.IsNil)
		c.Assert(m["gene"], gc.Equals, "simmons")
		c.Assert(m["meme"], gc.Equals, "foul-bachelor-frog")
	}
}

func (s *RelationUnitSuite) TestContainerSettings(c *gc.C) {
	prr := NewProReqRelation(c, &s.ConnSuite, charm.ScopeContainer)
	rus := RUs{prr.pru0, prr.pru1, prr.rru0, prr.rru1}

	// Check missing settings cannot be read by any RU.
	for _, ru := range rus {
		_, err := ru.ReadSettings("logging/0")
		c.Assert(err, gc.ErrorMatches, `cannot read settings for unit "logging/0" in relation "logging:info mysql:juju-info": settings not found`)
	}

	// Add settings for one RU.
	s.assertInScope(c, prr.pru0, false)
	err := prr.pru0.EnterScope(map[string]interface{}{"gene": "hackman"})
	c.Assert(err, gc.IsNil)
	node, err := prr.pru0.Settings()
	c.Assert(err, gc.IsNil)
	node.Set("meme", "foul-bachelor-frog")
	_, err = node.Write()
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, prr.pru0, true)

	// Check settings can be read by RUs in the same container.
	rus0 := RUs{prr.pru0, prr.rru0}
	for _, ru := range rus0 {
		m, err := ru.ReadSettings("mysql/0")
		c.Assert(err, gc.IsNil)
		c.Assert(m["gene"], gc.Equals, "hackman")
		c.Assert(m["meme"], gc.Equals, "foul-bachelor-frog")
	}

	// Check settings are still inaccessible to RUs outside that container
	rus1 := RUs{prr.pru1, prr.rru1}
	for _, ru := range rus1 {
		_, err := ru.ReadSettings("mysql/0")
		c.Assert(err, gc.ErrorMatches, `cannot read settings for unit "mysql/0" in relation "logging:info mysql:juju-info": settings not found`)
	}
}

func (s *RelationUnitSuite) TestContainerCreateSubordinate(c *gc.C) {
	psvc, err := s.State.AddService("mysql", s.AddTestingCharm(c, "mysql"))
	c.Assert(err, gc.IsNil)
	rsvc, err := s.State.AddService("logging", s.AddTestingCharm(c, "logging"))
	c.Assert(err, gc.IsNil)
	eps, err := s.State.InferEndpoints([]string{"mysql", "logging"})
	c.Assert(err, gc.IsNil)
	rel, err := s.State.AddRelation(eps...)
	c.Assert(err, gc.IsNil)
	punit, err := psvc.AddUnit()
	c.Assert(err, gc.IsNil)
	pru, err := rel.Unit(punit)
	c.Assert(err, gc.IsNil)

	// Check that no units of the subordinate service exist.
	assertSubCount := func(expect int) []*state.Unit {
		runits, err := rsvc.AllUnits()
		c.Assert(err, gc.IsNil)
		c.Assert(runits, gc.HasLen, expect)
		return runits
	}
	assertSubCount(0)

	// Enter principal's scope and check a subordinate was created.
	s.assertInScope(c, pru, false)
	err = pru.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	assertSubCount(1)
	s.assertInScope(c, pru, true)

	// Enter principal scope again and check no more subordinates created.
	err = pru.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	assertSubCount(1)
	s.assertInScope(c, pru, true)

	// Leave principal scope, then re-enter, and check that still no further
	// subordinates are created.
	err = pru.LeaveScope()
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, pru, false)
	err = pru.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	runits := assertSubCount(1)
	s.assertInScope(c, pru, true)

	// Set the subordinate to Dying, and enter scope again; because the scope
	// is already entered, no error is returned.
	runit := runits[0]
	err = runit.Destroy()
	c.Assert(err, gc.IsNil)
	err = pru.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, pru, true)

	// Leave scope, then try to enter again with the Dying subordinate.
	err = pru.LeaveScope()
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, pru, false)
	err = pru.EnterScope(nil)
	c.Assert(err, gc.Equals, state.ErrCannotEnterScopeYet)
	s.assertInScope(c, pru, false)

	// Remove the subordinate, and enter scope again; this should work, and
	// create a new subordinate.
	err = runit.EnsureDead()
	c.Assert(err, gc.IsNil)
	err = runit.Remove()
	c.Assert(err, gc.IsNil)
	assertSubCount(0)
	s.assertInScope(c, pru, false)
	err = pru.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	assertSubCount(1)
	s.assertInScope(c, pru, true)
}

func (s *RelationUnitSuite) TestDestroyRelationWithUnitsInScope(c *gc.C) {
	pr := NewPeerRelation(c, s.State)
	rel := pr.ru0.Relation()

	// Enter two units, and check that Destroying the service sets the
	// relation to Dying (rather than removing it directly).
	s.assertInScope(c, pr.ru0, false)
	err := pr.ru0.EnterScope(map[string]interface{}{"some": "settings"})
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, pr.ru0, true)
	s.assertInScope(c, pr.ru1, false)
	err = pr.ru1.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, pr.ru1, true)
	err = pr.svc.Destroy()
	c.Assert(err, gc.IsNil)
	err = rel.Refresh()
	c.Assert(err, gc.IsNil)
	c.Assert(rel.Life(), gc.Equals, state.Dying)

	// Check that we can't add a new unit now.
	s.assertInScope(c, pr.ru2, false)
	err = pr.ru2.EnterScope(nil)
	c.Assert(err, gc.Equals, state.ErrCannotEnterScope)
	s.assertInScope(c, pr.ru2, false)

	// Check that we created no settings for the unit we failed to add.
	_, err = pr.ru0.ReadSettings("riak/2")
	c.Assert(err, gc.ErrorMatches, `cannot read settings for unit "riak/2" in relation "riak:ring": settings not found`)

	// ru0 leaves the scope; check that service Destroy is still a no-op.
	s.assertInScope(c, pr.ru0, true)
	err = pr.ru0.LeaveScope()
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, pr.ru0, false)
	err = pr.svc.Destroy()
	c.Assert(err, gc.IsNil)

	// Check that unit settings for the original unit still exist, and have
	// not yet been marked for deletion.
	err = s.State.Cleanup()
	c.Assert(err, gc.IsNil)
	assertSettings := func() {
		settings, err := pr.ru1.ReadSettings("riak/0")
		c.Assert(err, gc.IsNil)
		c.Assert(settings, gc.DeepEquals, map[string]interface{}{"some": "settings"})
	}
	assertSettings()

	// The final unit leaves the scope, and cleans up after itself.
	s.assertInScope(c, pr.ru1, true)
	err = pr.ru1.LeaveScope()
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, pr.ru1, false)
	err = rel.Refresh()
	c.Assert(err, checkers.Satisfies, errors.IsNotFoundError)

	// The settings were not themselves actually deleted yet...
	assertSettings()

	// ...but they were scheduled for deletion.
	err = s.State.Cleanup()
	c.Assert(err, gc.IsNil)
	_, err = pr.ru1.ReadSettings("riak/0")
	c.Assert(err, gc.ErrorMatches, `cannot read settings for unit "riak/0" in relation "riak:ring": settings not found`)

	// Because this is the only sensible place, check that a further call
	// to Cleanup does not error out.
	err = s.State.Cleanup()
	c.Assert(err, gc.IsNil)
}

func (s *RelationUnitSuite) TestAliveRelationScope(c *gc.C) {
	pr := NewPeerRelation(c, s.State)
	rel := pr.ru0.Relation()

	// Two units enter...
	s.assertInScope(c, pr.ru0, false)
	err := pr.ru0.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, pr.ru0, true)
	s.assertInScope(c, pr.ru1, false)
	err = pr.ru1.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, pr.ru1, true)

	// One unit becomes Dying, then re-enters the scope; this is not an error,
	// because the state is already as requested.
	err = pr.u0.Destroy()
	c.Assert(err, gc.IsNil)
	err = pr.ru0.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, pr.ru0, true)

	// Two units leave...
	err = pr.ru0.LeaveScope()
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, pr.ru0, false)
	err = pr.ru1.LeaveScope()
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, pr.ru1, false)

	// The relation scope is empty, but the relation is still alive...
	err = rel.Refresh()
	c.Assert(err, gc.IsNil)
	c.Assert(rel.Life(), gc.Equals, state.Alive)

	// ...and new units can still join it...
	s.assertInScope(c, pr.ru2, false)
	err = pr.ru2.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, pr.ru2, true)

	// ...but Dying units cannot.
	err = pr.u3.Destroy()
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, pr.ru3, false)
	err = pr.ru3.EnterScope(nil)
	c.Assert(err, gc.Equals, state.ErrCannotEnterScope)
	s.assertInScope(c, pr.ru3, false)
}

func (s *StateSuite) TestWatchWatchScopeDiesOnStateClose(c *gc.C) {
	testWatcherDiesWhenStateCloses(c, func(c *gc.C, st *state.State) waiter {
		pr := NewPeerRelation(c, st)
		w := pr.ru0.WatchScope()
		<-w.Changes()
		return w
	})
}

func (s *RelationUnitSuite) TestPeerWatchScope(c *gc.C) {
	pr := NewPeerRelation(c, s.State)

	// Test empty initial event.
	w0 := pr.ru0.WatchScope()
	defer testing.AssertStop(c, w0)
	s.assertScopeChange(c, w0, nil, nil)
	s.assertNoScopeChange(c, w0)

	// ru0 enters; check no change, but settings written.
	s.assertInScope(c, pr.ru0, false)
	err := pr.ru0.EnterScope(map[string]interface{}{"foo": "bar"})
	c.Assert(err, gc.IsNil)
	s.assertNoScopeChange(c, w0)
	node, err := pr.ru0.Settings()
	c.Assert(err, gc.IsNil)
	c.Assert(node.Map(), gc.DeepEquals, map[string]interface{}{"foo": "bar"})
	s.assertInScope(c, pr.ru0, true)

	// ru1 enters; check change is observed.
	s.assertInScope(c, pr.ru1, false)
	err = pr.ru1.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	s.assertScopeChange(c, w0, []string{"riak/1"}, nil)
	s.assertNoScopeChange(c, w0)
	s.assertInScope(c, pr.ru1, true)

	// ru1 enters again, check no problems and no changes.
	err = pr.ru1.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	s.assertNoScopeChange(c, w0)
	s.assertInScope(c, pr.ru1, true)

	// Stop watching; ru2 enters.
	testing.AssertStop(c, w0)
	s.assertInScope(c, pr.ru2, false)
	err = pr.ru2.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, pr.ru2, true)

	// Start watch again, check initial event.
	w0 = pr.ru0.WatchScope()
	defer testing.AssertStop(c, w0)
	s.assertScopeChange(c, w0, []string{"riak/1", "riak/2"}, nil)
	s.assertNoScopeChange(c, w0)

	// ru1 leaves; check event.
	s.assertInScope(c, pr.ru1, true)
	err = pr.ru1.LeaveScope()
	c.Assert(err, gc.IsNil)
	s.assertScopeChange(c, w0, nil, []string{"riak/1"})
	s.assertNoScopeChange(c, w0)
	s.assertInScope(c, pr.ru1, false)

	// ru1 leaves again; check no problems and no changes.
	err = pr.ru1.LeaveScope()
	c.Assert(err, gc.IsNil)
	s.assertNoScopeChange(c, w0)
	s.assertInScope(c, pr.ru1, false)
}

func (s *RelationUnitSuite) TestProReqWatchScope(c *gc.C) {
	prr := NewProReqRelation(c, &s.ConnSuite, charm.ScopeGlobal)

	// Test empty initial events for all RUs.
	ws := prr.watches()
	for _, w := range ws {
		defer testing.AssertStop(c, w)
	}
	for _, w := range ws {
		s.assertScopeChange(c, w, nil, nil)
	}
	s.assertNoScopeChange(c, ws...)

	// pru0 enters; check detected only by req RUs.
	s.assertInScope(c, prr.pru0, false)
	err := prr.pru0.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	rws := func() []*state.RelationScopeWatcher {
		return []*state.RelationScopeWatcher{ws[2], ws[3]}
	}
	for _, w := range rws() {
		s.assertScopeChange(c, w, []string{"mysql/0"}, nil)
	}
	s.assertNoScopeChange(c, ws...)
	s.assertInScope(c, prr.pru0, true)

	// req0 enters; check detected only by pro RUs.
	s.assertInScope(c, prr.rru0, false)
	err = prr.rru0.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	pws := func() []*state.RelationScopeWatcher {
		return []*state.RelationScopeWatcher{ws[0], ws[1]}
	}
	for _, w := range pws() {
		s.assertScopeChange(c, w, []string{"wordpress/0"}, nil)
	}
	s.assertNoScopeChange(c, ws...)
	s.assertInScope(c, prr.rru0, true)

	// Stop watches; remaining RUs enter.
	for _, w := range ws {
		testing.AssertStop(c, w)
	}
	s.assertInScope(c, prr.pru1, false)
	err = prr.pru1.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, prr.pru1, true)
	s.assertInScope(c, prr.rru1, false)
	err = prr.rru1.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, prr.rru0, true)

	// Start new watches, check initial events.
	ws = prr.watches()
	for _, w := range ws {
		defer testing.AssertStop(c, w)
	}
	for _, w := range pws() {
		s.assertScopeChange(c, w, []string{"wordpress/0", "wordpress/1"}, nil)
	}
	for _, w := range rws() {
		s.assertScopeChange(c, w, []string{"mysql/0", "mysql/1"}, nil)
	}
	s.assertNoScopeChange(c, ws...)

	// pru0 leaves; check detected only by req RUs.
	s.assertInScope(c, prr.pru0, true)
	err = prr.pru0.LeaveScope()
	c.Assert(err, gc.IsNil)
	for _, w := range rws() {
		s.assertScopeChange(c, w, nil, []string{"mysql/0"})
	}
	s.assertNoScopeChange(c, ws...)
	s.assertInScope(c, prr.pru0, false)

	// rru0 leaves; check detected only by pro RUs.
	s.assertInScope(c, prr.rru0, true)
	err = prr.rru0.LeaveScope()
	c.Assert(err, gc.IsNil)
	for _, w := range pws() {
		s.assertScopeChange(c, w, nil, []string{"wordpress/0"})
	}
	s.assertNoScopeChange(c, ws...)
	s.assertInScope(c, prr.rru0, false)
}

func (s *RelationUnitSuite) TestContainerWatchScope(c *gc.C) {
	prr := NewProReqRelation(c, &s.ConnSuite, charm.ScopeContainer)

	// Test empty initial events for all RUs.
	ws := prr.watches()
	for _, w := range ws {
		defer testing.AssertStop(c, w)
	}
	for _, w := range ws {
		s.assertScopeChange(c, w, nil, nil)
	}
	s.assertNoScopeChange(c, ws...)

	// pru0 enters; check detected only by same-container req.
	s.assertInScope(c, prr.pru0, false)
	err := prr.pru0.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	s.assertScopeChange(c, ws[2], []string{"mysql/0"}, nil)
	s.assertNoScopeChange(c, ws...)
	s.assertInScope(c, prr.pru0, true)

	// req1 enters; check detected only by same-container pro.
	s.assertInScope(c, prr.rru1, false)
	err = prr.rru1.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	s.assertScopeChange(c, ws[1], []string{"logging/1"}, nil)
	s.assertNoScopeChange(c, ws...)
	s.assertInScope(c, prr.rru1, true)

	// Stop watches; remaining RUs enter scope.
	for _, w := range ws {
		testing.AssertStop(c, w)
	}
	s.assertInScope(c, prr.pru1, false)
	err = prr.pru1.EnterScope(nil)
	c.Assert(err, gc.IsNil)
	s.assertInScope(c, prr.rru0, false)
	err = prr.rru0.EnterScope(nil)
	c.Assert(err, gc.IsNil)

	// Start new watches, check initial events.
	ws = prr.watches()
	for _, w := range ws {
		defer testing.AssertStop(c, w)
	}
	s.assertScopeChange(c, ws[0], []string{"logging/0"}, nil)
	s.assertScopeChange(c, ws[1], []string{"logging/1"}, nil)
	s.assertScopeChange(c, ws[2], []string{"mysql/0"}, nil)
	s.assertScopeChange(c, ws[3], []string{"mysql/1"}, nil)
	s.assertNoScopeChange(c, ws...)
	s.assertInScope(c, prr.pru1, true)
	s.assertInScope(c, prr.rru0, true)

	// pru0 leaves; check detected only by same-container req.
	s.assertInScope(c, prr.pru0, true)
	err = prr.pru0.LeaveScope()
	c.Assert(err, gc.IsNil)
	s.assertScopeChange(c, ws[2], nil, []string{"mysql/0"})
	s.assertNoScopeChange(c, ws...)
	s.assertInScope(c, prr.pru0, false)

	// rru0 leaves; check detected only by same-container pro.
	s.assertInScope(c, prr.rru0, true)
	err = prr.rru0.LeaveScope()
	c.Assert(err, gc.IsNil)
	s.assertScopeChange(c, ws[0], nil, []string{"logging/0"})
	s.assertNoScopeChange(c, ws...)
	s.assertInScope(c, prr.rru0, false)
}

func (s *RelationUnitSuite) assertScopeChange(c *gc.C, w *state.RelationScopeWatcher, entered, left []string) {
	s.State.StartSync()
	select {
	case ch, ok := <-w.Changes():
		c.Assert(ok, gc.Equals, true)
		sort.Strings(entered)
		sort.Strings(ch.Entered)
		c.Assert(ch.Entered, gc.DeepEquals, entered)
		sort.Strings(left)
		sort.Strings(ch.Left)
		c.Assert(ch.Left, gc.DeepEquals, left)
	case <-time.After(coretesting.LongWait):
		c.Fatalf("no change")
	}
}

func (s *RelationUnitSuite) assertNoScopeChange(c *gc.C, ws ...*state.RelationScopeWatcher) {
	s.State.StartSync()
	for _, w := range ws {
		select {
		case ch, ok := <-w.Changes():
			c.Fatalf("got unwanted change: %#v, %t", ch, ok)
		case <-time.After(coretesting.ShortWait):
		}
	}
}

type PeerRelation struct {
	rel                *state.Relation
	svc                *state.Service
	u0, u1, u2, u3     *state.Unit
	ru0, ru1, ru2, ru3 *state.RelationUnit
}

func NewPeerRelation(c *gc.C, st *state.State) *PeerRelation {
	svc, err := st.AddService("riak", state.AddTestingCharm(c, st, "riak"))
	c.Assert(err, gc.IsNil)
	ep, err := svc.Endpoint("ring")
	c.Assert(err, gc.IsNil)
	rel, err := st.EndpointsRelation(ep)
	c.Assert(err, gc.IsNil)
	pr := &PeerRelation{rel: rel, svc: svc}
	pr.u0, pr.ru0 = addRU(c, svc, rel, nil)
	pr.u1, pr.ru1 = addRU(c, svc, rel, nil)
	pr.u2, pr.ru2 = addRU(c, svc, rel, nil)
	pr.u3, pr.ru3 = addRU(c, svc, rel, nil)
	return pr
}

type ProReqRelation struct {
	rel                    *state.Relation
	psvc, rsvc             *state.Service
	pu0, pu1, ru0, ru1     *state.Unit
	pru0, pru1, rru0, rru1 *state.RelationUnit
}

func NewProReqRelation(c *gc.C, s *ConnSuite, scope charm.RelationScope) *ProReqRelation {
	psvc, err := s.State.AddService("mysql", s.AddTestingCharm(c, "mysql"))
	c.Assert(err, gc.IsNil)
	var rsvc *state.Service
	if scope == charm.ScopeGlobal {
		rsvc, err = s.State.AddService("wordpress", s.AddTestingCharm(c, "wordpress"))
	} else {
		rsvc, err = s.State.AddService("logging", s.AddTestingCharm(c, "logging"))
	}
	c.Assert(err, gc.IsNil)
	eps, err := s.State.InferEndpoints([]string{"mysql", rsvc.Name()})
	c.Assert(err, gc.IsNil)
	rel, err := s.State.AddRelation(eps...)
	c.Assert(err, gc.IsNil)
	prr := &ProReqRelation{rel: rel, psvc: psvc, rsvc: rsvc}
	prr.pu0, prr.pru0 = addRU(c, psvc, rel, nil)
	prr.pu1, prr.pru1 = addRU(c, psvc, rel, nil)
	if scope == charm.ScopeGlobal {
		prr.ru0, prr.rru0 = addRU(c, rsvc, rel, nil)
		prr.ru1, prr.rru1 = addRU(c, rsvc, rel, nil)
	} else {
		prr.ru0, prr.rru0 = addRU(c, rsvc, rel, prr.pu0)
		prr.ru1, prr.rru1 = addRU(c, rsvc, rel, prr.pu1)
	}
	return prr
}

func (prr *ProReqRelation) watches() []*state.RelationScopeWatcher {
	return []*state.RelationScopeWatcher{
		prr.pru0.WatchScope(), prr.pru1.WatchScope(),
		prr.rru0.WatchScope(), prr.rru1.WatchScope(),
	}
}

func addRU(c *gc.C, svc *state.Service, rel *state.Relation, principal *state.Unit) (*state.Unit, *state.RelationUnit) {
	// Given the service svc in the relation rel, add a unit of svc and create
	// a RelationUnit with rel. If principal is supplied, svc is assumed to be
	// subordinate and the unit will be created by temporarily entering the
	// relation's scope as the principal.
	var u *state.Unit
	if principal == nil {
		unit, err := svc.AddUnit()
		c.Assert(err, gc.IsNil)
		u = unit
	} else {
		origUnits, err := svc.AllUnits()
		c.Assert(err, gc.IsNil)
		pru, err := rel.Unit(principal)
		c.Assert(err, gc.IsNil)
		err = pru.EnterScope(nil) // to create the subordinate
		c.Assert(err, gc.IsNil)
		err = pru.LeaveScope() // to reset to initial expected state
		c.Assert(err, gc.IsNil)
		newUnits, err := svc.AllUnits()
		c.Assert(err, gc.IsNil)
		for _, unit := range newUnits {
			found := false
			for _, old := range origUnits {
				if unit.Name() == old.Name() {
					found = true
					break
				}
			}
			if !found {
				u = unit
				break
			}
		}
		c.Assert(u, gc.NotNil)
	}
	preventUnitDestroyRemove(c, u)
	ru, err := rel.Unit(u)
	c.Assert(err, gc.IsNil)
	return u, ru
}

type RUs []*state.RelationUnit
