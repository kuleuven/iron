//nolint:forcetypeassert
package iron

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/kuleuven/iron/api"
	"github.com/kuleuven/iron/msg"
)

// mockPoolConn is a minimal Conn implementation for pool tests.
type mockPoolConn struct {
	connectedAt     time.Time
	transportErrors int
	sqlErrors       int
	closed          bool
	closeMu         sync.Mutex
}

func newMockPoolConn() *mockPoolConn {
	return &mockPoolConn{
		connectedAt: time.Now(),
	}
}

func (m *mockPoolConn) Env() Env                { return Env{} }
func (m *mockPoolConn) Transport() net.Conn     { return nil }
func (m *mockPoolConn) ServerVersion() string   { return "4.3.2" }
func (m *mockPoolConn) ClientSignature() string { return "sig" }
func (m *mockPoolConn) NativePassword() string  { return "pw" }
func (m *mockPoolConn) ConnectedAt() time.Time  { return m.connectedAt }
func (m *mockPoolConn) TransportErrors() int    { return m.transportErrors }
func (m *mockPoolConn) SQLErrors() int          { return m.sqlErrors }
func (m *mockPoolConn) Request(_ context.Context, _ msg.APINumber, _, _ any) error {
	return nil
}

func (m *mockPoolConn) RequestWithBuffers(_ context.Context, _ msg.APINumber, _, _ any, _, _ []byte) error {
	return nil
}
func (m *mockPoolConn) API() *api.API { return nil }
func (m *mockPoolConn) Close() error {
	m.closeMu.Lock()
	defer m.closeMu.Unlock()

	m.closed = true

	return nil
}

func (m *mockPoolConn) RegisterCloseHandler(_ func() error) context.CancelFunc {
	return func() {}
}

// newTestClient creates a Client with a mock HandshakeFunc that returns mockPoolConns.
func newTestClient(maxConns int) *Client {
	env := Env{
		Username: "testUser",
		Zone:     "testZone",
	}

	client := &Client{
		env:      &env,
		protocol: msg.XML,
		option: Option{
			MaxConns:   maxConns,
			ClientName: "test",
			HandshakeFunc: func(ctx context.Context) (Conn, error) {
				return newMockPoolConn(), nil
			},
		},
	}

	client.defaultPool = newPool(client)
	client.API = client.defaultPool.API

	return client
}

func TestPoolConnectAndReturn(t *testing.T) {
	client := newTestClient(2)
	defer client.Close()

	ctx := t.Context()

	conn1, err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	conn2, err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both connections should exist
	if conn1 == nil || conn2 == nil {
		t.Fatal("expected non-nil connections")
	}

	// Return first connection
	if err := conn1.Close(); err != nil {
		t.Fatalf("unexpected error closing conn1: %v", err)
	}

	// Should be able to get another connection (reuses the returned one)
	conn3, err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := conn2.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := conn3.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPoolConnectAvailable(t *testing.T) {
	client := newTestClient(3)
	defer client.Close()

	ctx := t.Context()

	// No connections yet, but ConnectAvailable should create new ones
	conns, err := client.ConnectAvailable(ctx, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(conns) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(conns))
	}

	// Return them
	for _, c := range conns {
		c.Close()
	}

	// Now get all available (should include the 2 returned ones)
	conns, err = client.ConnectAvailable(ctx, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(conns) != 3 {
		t.Fatalf("expected 3 connections (2 reused + 1 new), got %d", len(conns))
	}

	for _, c := range conns {
		c.Close()
	}
}

func TestPoolConnectAvailableEmpty(t *testing.T) {
	client := newTestClient(2)
	defer client.Close()

	ctx := t.Context()

	// Use all connections
	conn1, err := client.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	conn2, err := client.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// No more available
	conns, err := client.ConnectAvailable(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}

	if len(conns) != 0 {
		t.Fatalf("expected 0 connections, got %d", len(conns))
	}

	conn1.Close()
	conn2.Close()
}

func TestPoolClose(t *testing.T) {
	client := newTestClient(2)

	ctx := t.Context()

	conn1, err := client.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	conn1.Close()

	if err := client.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Double close should be safe
	if err := client.Close(); err != nil {
		t.Fatalf("unexpected error on double close: %v", err)
	}
}

func TestPoolConcurrentUse(t *testing.T) {
	env := Env{
		Username: "testUser",
		Zone:     "testZone",
	}

	client := &Client{
		env:      &env,
		protocol: msg.XML,
		option: Option{
			MaxConns:           1,
			AllowConcurrentUse: true,
			ClientName:         "test",
			HandshakeFunc: func(ctx context.Context) (Conn, error) {
				return newMockPoolConn(), nil
			},
		},
	}

	client.defaultPool = newPool(client)
	client.API = client.defaultPool.API

	defer client.Close()

	ctx := t.Context()

	// Connect first to create a connection
	conn1, err := client.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Second connect should reuse same connection since AllowConcurrentUse=true
	conn2, err := client.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	conn1.Close()
	conn2.Close()
}

func TestPoolSubpool(t *testing.T) {
	client := newTestClient(4)
	defer client.Close()

	ctx := t.Context()

	// Pre-create a connection so the subpool can take it
	conn, err := client.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	conn.Close()

	sub, err := client.Pool(2)
	if err != nil {
		t.Fatal(err)
	}

	// Sub pool should be usable
	subConn, err := sub.Connect(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	subConn.Close()

	// Close subpool should return connections to parent
	if err := sub.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPoolSubpoolTooLarge(t *testing.T) {
	client := newTestClient(2)
	defer client.Close()

	_, err := client.Pool(5)
	if err == nil {
		t.Fatal("expected error for subpool larger than parent")
	}

	if !errors.Is(err, ErrNoConnectionsAvailable) {
		t.Fatalf("expected ErrNoConnectionsAvailable, got %v", err)
	}
}

func TestPoolSubpoolDefaultSize(t *testing.T) {
	client := newTestClient(3)
	defer client.Close()

	sub, err := client.Pool(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer sub.Close()

	// Size 0 means use parent's maxConns
	ctx := t.Context()

	conn, err := sub.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	conn.Close()
}

func TestReplaceDefaultPool(t *testing.T) {
	client := newTestClient(2)
	defer client.Close()

	newPool := newPool(client)
	newPool.maxConns = 5

	old := client.ReplaceDefaultPool(newPool)

	if old == nil {
		t.Fatal("expected non-nil old pool")
	}

	// New pool should be in effect
	if client.defaultPool != newPool {
		t.Error("expected defaultPool to be replaced")
	}

	old.Close()
}

func TestReturnOnCloseIdempotent(t *testing.T) {
	client := newTestClient(1)
	defer client.Close()

	ctx := t.Context()

	conn, err := client.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// First close
	if err := conn.Close(); err != nil {
		t.Fatalf("unexpected error on first close: %v", err)
	}

	// Second close should not error (idempotent via sync.Once)
	if err := conn.Close(); err != nil {
		t.Fatalf("unexpected error on second close: %v", err)
	}
}

func TestDiscardConnectionOnTransportError(t *testing.T) {
	client := newTestClient(2)
	defer client.Close()

	ctx := t.Context()

	conn, err := client.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate a transport error on the underlying connection
	roc := conn.(*returnOnClose)
	mc := roc.Conn.(*mockPoolConn)
	mc.transportErrors = 1

	// Returning a connection with errors should discard it
	conn.Close()

	// Pool should have 0 available connections since it was discarded
	client.defaultPool.lock.Lock()
	availCount := len(client.defaultPool.available)
	allCount := len(client.defaultPool.all)
	client.defaultPool.lock.Unlock()

	if availCount != 0 {
		t.Errorf("expected 0 available connections, got %d", availCount)
	}

	if allCount != 0 {
		t.Errorf("expected 0 total connections (discarded), got %d", allCount)
	}
}

func TestDiscardConnectionOnSQLError(t *testing.T) {
	client := newTestClient(2)
	defer client.Close()

	ctx := t.Context()

	conn, err := client.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate a SQL error
	roc := conn.(*returnOnClose)
	mc := roc.Conn.(*mockPoolConn)
	mc.sqlErrors = 1

	conn.Close()

	client.defaultPool.lock.Lock()
	allCount := len(client.defaultPool.all)
	client.defaultPool.lock.Unlock()

	if allCount != 0 {
		t.Errorf("expected 0 total connections (discarded due to SQL error), got %d", allCount)
	}
}

func TestDiscardOldConnections(t *testing.T) {
	env := Env{
		Username: "testUser",
		Zone:     "testZone",
	}

	client := &Client{
		env:      &env,
		protocol: msg.XML,
		option: Option{
			MaxConns:             3,
			ClientName:           "test",
			DiscardConnectionAge: 100 * time.Millisecond,
			HandshakeFunc: func(ctx context.Context) (Conn, error) {
				return newMockPoolConn(), nil
			},
		},
	}

	client.defaultPool = newPool(client)
	client.API = client.defaultPool.API

	defer client.Close()

	// Directly add an old connection to the pool's available and all slices
	oldConn := newMockPoolConn()
	oldConn.connectedAt = time.Now().Add(-1 * time.Second)

	client.defaultPool.lock.Lock()
	client.defaultPool.available = append(client.defaultPool.available, oldConn)
	client.defaultPool.all = append(client.defaultPool.all, oldConn)
	client.defaultPool.lock.Unlock()

	// Verify the connection is available
	client.defaultPool.lock.Lock()
	availBefore := len(client.defaultPool.available)
	client.defaultPool.lock.Unlock()

	if availBefore != 1 {
		t.Fatalf("expected 1 available connection before discard, got %d", availBefore)
	}

	// Run discard
	client.defaultPool.lock.Lock()
	client.defaultPool.discardOldConnections()
	availAfter := len(client.defaultPool.available)
	allAfter := len(client.defaultPool.all)
	client.defaultPool.lock.Unlock()

	if availAfter != 0 {
		t.Errorf("expected 0 available connections after discard, got %d", availAfter)
	}

	if allAfter != 0 {
		t.Errorf("expected 0 total connections after discard, got %d", allAfter)
	}
}

func TestDiscardOldConnectionsNotExpired(t *testing.T) {
	env := Env{
		Username: "testUser",
		Zone:     "testZone",
	}

	client := &Client{
		env:      &env,
		protocol: msg.XML,
		option: Option{
			MaxConns:             3,
			ClientName:           "test",
			DiscardConnectionAge: time.Hour,
			HandshakeFunc: func(ctx context.Context) (Conn, error) {
				return newMockPoolConn(), nil
			},
		},
	}

	client.defaultPool = newPool(client)
	client.API = client.defaultPool.API

	defer client.Close()

	ctx := t.Context()

	conn, err := client.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	conn.Close()

	// Run discard — connection is fresh, should not be removed
	client.defaultPool.lock.Lock()
	client.defaultPool.discardOldConnections()
	availAfter := len(client.defaultPool.available)
	client.defaultPool.lock.Unlock()

	if availAfter != 1 {
		t.Errorf("expected 1 available connection (not expired), got %d", availAfter)
	}
}

func TestDiscardOldConnectionsDisabled(t *testing.T) {
	client := newTestClient(2)
	defer client.Close()

	ctx := t.Context()

	conn, err := client.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	conn.Close()

	// discardConnectionAge is 0, so discardOldConnections should be a no-op
	client.defaultPool.lock.Lock()
	client.defaultPool.discardOldConnections()
	availAfter := len(client.defaultPool.available)
	client.defaultPool.lock.Unlock()

	if availAfter != 1 {
		t.Errorf("expected 1 available connection (discard disabled), got %d", availAfter)
	}
}

func TestCloseAll(t *testing.T) {
	conns := []Conn{
		newMockPoolConn(),
		newMockPoolConn(),
		newMockPoolConn(),
	}

	err := closeAll(conns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, c := range conns {
		if !c.(*mockPoolConn).closed {
			t.Errorf("connection %d not closed", i)
		}
	}
}

func TestCloseAllEmpty(t *testing.T) {
	err := closeAll(nil)
	if err != nil {
		t.Fatalf("unexpected error for nil slice: %v", err)
	}

	err = closeAll([]Conn{})
	if err != nil {
		t.Fatalf("unexpected error for empty slice: %v", err)
	}
}

func TestPoolBlockingConnect(t *testing.T) {
	client := newTestClient(1)
	defer client.Close()

	ctx := t.Context()

	// Take the only connection
	conn1, err := client.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Start a goroutine that will block waiting for a connection
	done := make(chan error, 1)
	go func() {
		conn2, err := client.Connect(ctx)
		if err != nil {
			done <- err
			return
		}

		conn2.Close()

		done <- nil
	}()

	// Give the goroutine time to start blocking
	time.Sleep(50 * time.Millisecond)

	// Return the connection so the blocked goroutine completes
	conn1.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("blocked goroutine got error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("blocked goroutine did not complete in time")
	}
}

func TestPoolChildClose(t *testing.T) {
	client := newTestClient(4)
	defer client.Close()

	ctx := t.Context()

	sub, err := client.Pool(2)
	if err != nil {
		t.Fatal(err)
	}

	// Use the subpool
	conn, err := sub.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	conn.Close()

	// Close the subpool — should return capacity to parent
	client.defaultPool.lock.Lock()
	parentMaxBefore := client.defaultPool.maxConns
	client.defaultPool.lock.Unlock()

	sub.Close()

	client.defaultPool.lock.Lock()
	parentMaxAfter := client.defaultPool.maxConns
	client.defaultPool.lock.Unlock()

	if parentMaxAfter <= parentMaxBefore {
		t.Errorf("expected parent maxConns to increase after subpool close, before=%d after=%d", parentMaxBefore, parentMaxAfter)
	}
}

func TestClientOption(t *testing.T) {
	client := newTestClient(2)
	defer client.Close()

	opt := client.Option()
	if opt.MaxConns != 2 {
		t.Errorf("expected MaxConns=2, got %d", opt.MaxConns)
	}

	if opt.ClientName != "test" {
		t.Errorf("expected ClientName='test', got %q", opt.ClientName)
	}
}
