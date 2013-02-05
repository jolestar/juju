package api

import (
	"code.google.com/p/go.net/websocket"
	"launchpad.net/juju-core/state"
	"fmt"
	"sync"
	"errors"
)

var (
	errBadId       = errors.New("id not found")
	errBadCreds    = errors.New("invalid entity name or password")
	errNotLoggedIn = errors.New("not logged in")
	errPerm = errors.New("permission denied")
)

// srvRoot represents a single client's connection to the state.
type srvRoot struct {
	admin *srvAdmin
	srv   *Server
	conn  *websocket.Conn

	user authUser
}

type srvAdmin struct {
	root *srvRoot
}

type srvMachine struct {
	root *srvRoot
	m    *state.Machine
}

type srvUnit struct {
	root *srvRoot
	u    *state.Unit
}

type srvUser struct {
	root *srvRoot
	u    *state.User
}

func newStateServer(srv *Server, conn *websocket.Conn) *srvRoot {
	r := &srvRoot{
		srv:  srv,
		conn: conn,
	}
	r.admin = &srvAdmin{
		root: r,
	}
	return r
}

func (r *srvRoot) Admin(id string) (*srvAdmin, error) {
	if id != "" {
		// Safeguard id for possible future use.
		return nil, errBadId
	}
	return r.admin, nil
}

func (r *srvRoot) Machine(id string) (*srvMachine, error) {
	if !r.user.loggedIn() {
		return nil, errNotLoggedIn
	}
	m, err := r.srv.state.Machine(id)
	if err != nil {
		return nil, err
	}
	return &srvMachine{
		root: r,
		m: m,
	}, nil
}

func (r *srvRoot) Unit(name string) (*srvUnit, error) {
	if !r.user.loggedIn() {
		return nil, errNotLoggedIn
	}
	u, err := r.srv.state.Unit(name)
	if err != nil {
		return nil, err
	}
	return &srvUnit{
		root: r,
		u: u,
	}, nil
}

func (r *srvRoot) User(name string) (*srvUser, error) {
	if !r.user.loggedIn() {
		return nil, errNotLoggedIn
	}
	u, err := r.srv.state.User(name)
	if err != nil {
		return nil, err
	}
	return &srvUser{
		root: r,
		u: u,
	}, nil
}

type rpcCreds struct {
	EntityName string
	Password   string
}

func (a *srvAdmin) Login(c rpcCreds) error {
	return a.root.user.login(a.root.srv.state, c.EntityName, c.Password)
}

type rpcMachine struct {
	InstanceId string
}

func (m *srvMachine) Get() (info rpcMachine) {
	instId, _ := m.m.InstanceId()
	info.InstanceId = string(instId)
	return
}

type rpcPassword struct {
	Password string
}

func setPassword(e state.AuthEntity, password string) error {
	// Catch expected common case of mispelled
	// or missing Password parameter.
	if password == "" {
		return fmt.Errorf("password is empty")
	}
	return e.SetPassword(password)
}

func (m *srvMachine) SetPassword(p rpcPassword) error {
	// Allow:
	// - the machine itself.
	// - the environment manager.
	allow := m.root.user.entityName() == m.m.EntityName() ||
		m.root.user.isMachineWithJob(state.JobManageEnviron)
	if !allow {
		return errPerm
	}
	return setPassword(m.m, p.Password)
}

func (u *srvUnit) SetPassword(p rpcPassword) error {
	ename := u.root.user.entityName()
	// Allow:
	// - the unit itself.
	// - the machine responsible for unit, if unit is principal
	// - the unit's principal unit, if unit is subordinate
	allow := ename != u.u.EntityName()
	if !allow {
		deployerName, ok := u.u.DeployerName()
		allow = ok && ename == deployerName
	}
	if !allow {
		return errPerm
	}
	return setPassword(u.u, p.Password)
}

func (u *srvUser) SetPassword(p rpcPassword) error {
	if u.root.user.entityName() != "user-admin" {
		return errPerm
	}
	return setPassword(u.u, p.Password)
}

type authUser struct {
	mu     sync.Mutex
	entity state.AuthEntity // logged-in entity
}

func (u *authUser) login(st *state.State, entityName, password string) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	entity, err := st.AuthEntity(entityName)
	if err != nil && !state.IsNotFound(err) {
		return err
	}
	// We return the same error when an entity
	// does not exist as for a bad password, so that
	// we don't allow unauthenticated users to find information
	// about existing entities.
	if err != nil || !entity.PasswordValid(password) {
		return errBadCreds
	}
	u.entity = entity
	return nil
}

func (u *authUser) loggedIn() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.entity != nil
}

func (u *authUser) entityName() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.entity.EntityName()
}

func (u *authUser) isMachineWithJob(j state.MachineJob) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	m, ok := u.entity.(*state.Machine)
	if !ok {
		return false
	}
	for _, mj := range m.Jobs() {
		if mj == j {
			return true
		}
	}
	return false
}
