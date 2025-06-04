package strava

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"stravach/app/storage/models"
)

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGetAllActivities_MultiplePages(t *testing.T) {
	calledPages := []int{}
	Handler = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		pageStr := req.URL.Query().Get("page")
		page, _ := strconv.Atoi(pageStr)
		calledPages = append(calledPages, page)
		var body string
		if page == 1 {
			body = `[{"id":1},{"id":2}]`
		} else {
			body = `[]`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	defer func() { Handler = &http.Client{} }()

	c := &Client{}
	activities, err := c.GetAllActivities("token")
	require.NoError(t, err)
	require.Len(t, *activities, 2)
	require.Equal(t, []int{1, 2}, calledPages)
}

func TestUpdateActivity_ReturnsUpdated(t *testing.T) {
	Handler = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":123,"name":"Updated"}`))}, nil
	})}
	defer func() { Handler = &http.Client{} }()

	c := &Client{}
	updated, err := c.UpdateActivity("token", models.UserActivity{ID: 123, Name: "Old"})
	require.NoError(t, err)
	require.Equal(t, "Updated", updated.Name)
}
