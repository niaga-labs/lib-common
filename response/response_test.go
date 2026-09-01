package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type row struct {
	ID string `json:"id"`
}

// record runs one handler and gives back the decoded body, so each case asserts on
// the JSON a client actually receives rather than on Go values.
func record(t *testing.T, handler gin.HandlerFunc) map[string]json.RawMessage {
	t.Helper()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/x", handler)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response was not JSON: %v (%s)", err, rec.Body.String())
	}
	return body
}

// The DMB-74 regression guards. A repository with no rows returns a nil slice, and
// encoding/json writes that as null — so `data.map()` blew up on an empty result and
// every probe and frontend needed a null guard.
func TestPaginated_NilSliceBecomesEmptyArray(t *testing.T) {
	var rows []row // nil, exactly what a repository returns for no rows

	body := record(t, func(c *gin.Context) {
		Paginated(c, rows, 1, 20, 0)
	})

	if got := string(body["data"]); got != "[]" {
		t.Fatalf("want data [], got %s", got)
	}
	if _, ok := body["meta"]; !ok {
		t.Fatal("pagination meta must still be present")
	}
}

func TestOK_NilSliceBecomesEmptyArray(t *testing.T) {
	var rows []row

	body := record(t, func(c *gin.Context) {
		OK(c, "", rows)
	})

	if got := string(body["data"]); got != "[]" {
		t.Fatalf("want data [], got %s", got)
	}
}

func TestOK_NilMapBecomesEmptyObject(t *testing.T) {
	var counts map[string]int

	body := record(t, func(c *gin.Context) {
		OK(c, "", counts)
	})

	if got := string(body["data"]); got != "{}" {
		t.Fatalf("want data {}, got %s", got)
	}
}

// A populated collection must pass through untouched.
func TestPaginated_NonEmptySliceIsUnchanged(t *testing.T) {
	body := record(t, func(c *gin.Context) {
		Paginated(c, []row{{ID: "a"}}, 1, 20, 1)
	})

	if got := string(body["data"]); got != `[{"id":"a"}]` {
		t.Fatalf("want the rows unchanged, got %s", got)
	}
}

// "The object you asked for does not exist" is genuinely null. Turning a nil pointer
// into [] would be a lie, so the helper leaves it alone.
func TestOK_NilPointerStaysNull(t *testing.T) {
	var missing *row

	body := record(t, func(c *gin.Context) {
		OK(c, "", missing)
	})

	if got := string(body["data"]); got != "null" {
		t.Fatalf("a missing object must stay null, got %s", got)
	}
}

// Deleted passes an untyped nil and must keep omitting data entirely.
func TestDeleted_UntypedNilIsUnaffected(t *testing.T) {
	body := record(t, func(c *gin.Context) {
		Deleted(c, "gone")
	})

	if raw, ok := body["data"]; ok && string(raw) != "null" {
		t.Fatalf("want data absent or null, got %s", string(raw))
	}
}
