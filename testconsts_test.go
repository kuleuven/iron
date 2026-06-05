package iron

// Shared string constants for tests in package iron. Defined to satisfy the
// goconst linter for repeated literals that don't already have a production
// constant.
const (
	testIP             = "127.0.0.1"
	testHost           = "localhost"
	testHostPort       = "localhost:5453"
	testStr            = "test"
	testPath           = "path"
	testPassword       = "password"
	testKey            = "key"
	testOldKey         = "old"
	testNewKey         = "/newkey"
	testCustom         = "custom"
	testNoNegotiation  = "no_negotiation"
	testCSNegRequire   = "CS_NEQ_REQUIRE"
	testJohnDoe        = "john_doe"
	testAdminPassword  = "admin_password"
	testZoneName       = "testZone"
	testUserName       = "testUser"
	testPasswordValue  = "testPassword"
	testNativePassword = "testNativePassword"
	testUserLower      = "testuser"
	testSecret         = "secret"
	testUsername       = "username"
)
