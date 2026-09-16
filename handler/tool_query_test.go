package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseToolFilterDefaultsAndAliases(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/tools?q=mysql&tag=sql,readonly&tags=demo&status=published", nil)
	c := testGinContext(req)

	filter, err := parseToolFilter(c)
	if err != nil {
		t.Fatalf("parseToolFilter returned error: %v", err)
	}
	if filter.Keyword != "mysql" {
		t.Fatalf("expected q alias to fill keyword, got %q", filter.Keyword)
	}
	if filter.Page != 1 || filter.PageSize != 20 {
		t.Fatalf("unexpected pagination defaults: page=%d page_size=%d", filter.Page, filter.PageSize)
	}
	if filter.Sort != "popularity" {
		t.Fatalf("unexpected default sort: %q", filter.Sort)
	}
	if filter.Published == nil || !*filter.Published {
		t.Fatalf("status=published should set published filter")
	}
	if len(filter.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %v", filter.Tags)
	}
}

func TestParseToolFilterRejectsInvalidPageSize(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/tools?page_size=101", nil)
	c := testGinContext(req)

	if _, err := parseToolFilter(c); err == nil {
		t.Fatal("expected page_size over 100 to be rejected")
	}
}

func TestParseToolFilterRejectsInvalidSort(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/tools?sort=bad", nil)
	c := testGinContext(req)

	if _, err := parseToolFilter(c); err == nil {
		t.Fatal("expected invalid sort to be rejected")
	}
}

func TestParseToolFilterAllowsAllMarketStatus(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/tools?status=all&published=false&page=2&page_size=10", nil)
	c := testGinContext(req)

	filter, err := parseToolFilter(c)
	if err != nil {
		t.Fatalf("parseToolFilter returned error: %v", err)
	}
	if filter.Status != "all" || filter.Published == nil || *filter.Published {
		t.Fatalf("unexpected status or published filter: status=%q published=%v", filter.Status, filter.Published)
	}
	if filter.Page != 2 || filter.PageSize != 10 {
		t.Fatalf("unexpected pagination: page=%d page_size=%d", filter.Page, filter.PageSize)
	}
}

func testGinContext(req *http.Request) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c
}
