package searchV2

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/api/routing"
	"github.com/grafana/grafana/pkg/middleware"
	"github.com/grafana/grafana/pkg/models"
	"github.com/prometheus/client_golang/prometheus"
)

type SearchHTTPService interface {
	RegisterHTTPRoutes(storageRoute routing.RouteRegister)
}

type searchHTTPService struct {
	search SearchService
}

func ProvideSearchHTTPService(search SearchService) SearchHTTPService {
	return &searchHTTPService{search: search}
}

func (s *searchHTTPService) RegisterHTTPRoutes(storageRoute routing.RouteRegister) {
	storageRoute.Post("/", middleware.ReqSignedIn, routing.Wrap(s.doQuery))
}

func (s *searchHTTPService) doQuery(c *models.ReqContext) response.Response {
	searchReadinessCheckResp := s.search.IsReady(c.Req.Context(), c.OrgID)
	if !searchReadinessCheckResp.IsReady {
		dashboardSearchNotServedRequestsCounter.With(prometheus.Labels{
			"reason": searchReadinessCheckResp.Reason,
		}).Inc()

		bytes, err := (&data.Frame{
			Name: "Loading",
		}).MarshalJSON()

		if err != nil {
			return response.Error(500, "error marshalling response", err)
		}
		return response.JSON(200, bytes)
	}

	body, err := io.ReadAll(c.Req.Body)
	if err != nil {
		return response.Error(500, "error reading bytes", err)
	}

	query := &DashboardQuery{}
	err = json.Unmarshal(body, query)
	if err != nil {
		return response.Error(400, "error parsing body", err)
	}

	resp := s.search.doDashboardQuery(c.Req.Context(), c.SignedInUser, c.OrgID, *query)

	if resp.Error != nil {
		return response.Error(500, "error handling search request", resp.Error)
	}

	if len(resp.Frames) != 1 {
		return response.Error(500, "invalid search response", errors.New("invalid search response"))
	}

	bytes, err := resp.Frames[0].MarshalJSON()
	if err != nil {
		return response.Error(500, "error marshalling response", err)
	}

	return response.JSON(200, bytes)
}
