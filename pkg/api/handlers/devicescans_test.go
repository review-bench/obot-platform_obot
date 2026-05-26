package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	apitypes "github.com/obot-platform/obot/apiclient/types"
	"github.com/obot-platform/obot/pkg/api"
	gclient "github.com/obot-platform/obot/pkg/gateway/client"
	gatewaydb "github.com/obot-platform/obot/pkg/gateway/db"
	gtypes "github.com/obot-platform/obot/pkg/gateway/types"
	sservices "github.com/obot-platform/obot/pkg/storage/services"
	kuser "k8s.io/apiserver/pkg/authentication/user"
)

// newDeviceScanTestGateway builds an in-memory sqlite-backed gateway
// client suitable for handler tests. It mirrors the unexported
// newTestClient helper in pkg/gateway/client and uses the package's
// public New constructor for parity with production wiring.
func newDeviceScanTestGateway(t *testing.T) *gclient.Client {
	t.Helper()

	services, err := sservices.New(sservices.Config{DSN: "sqlite://:memory:"})
	if err != nil {
		t.Fatalf("storage services: %v", err)
	}
	db, err := gatewaydb.New(services.DB.DB, services.DB.SQLDB, true)
	if err != nil {
		t.Fatalf("gateway db: %v", err)
	}
	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("auto-migrate: %v", err)
	}

	// gclient.New starts goroutines we don't need for these tests,
	// but it sticks to the package's public surface and is cheap.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return gclient.New(ctx, db, nil, nil, nil, nil, 0, 0, 0)
}

func insertTestDeviceScan(t *testing.T, c *gclient.Client, submittedBy, deviceID string) gtypes.DeviceScan {
	t.Helper()
	scan := gtypes.DeviceScan{SubmittedBy: submittedBy, DeviceID: deviceID}
	if err := c.InsertDeviceScan(context.Background(), &scan); err != nil {
		t.Fatalf("insert scan: %v", err)
	}
	return scan
}

func basicUser(uid string) *kuser.DefaultInfo {
	return &kuser.DefaultInfo{
		Name:   uid,
		UID:    uid,
		Groups: []string{apitypes.GroupBasic, apitypes.GroupAuthenticated},
	}
}

func adminUser(uid string) *kuser.DefaultInfo {
	return &kuser.DefaultInfo{
		Name:   uid,
		UID:    uid,
		Groups: []string{apitypes.GroupAdmin, apitypes.GroupAuthenticated},
	}
}

func auditorUser(uid string) *kuser.DefaultInfo {
	return &kuser.DefaultInfo{
		Name:   uid,
		UID:    uid,
		Groups: []string{apitypes.GroupAuditor, apitypes.GroupAuthenticated},
	}
}

func TestDeviceScansList_BasicUserSeesOnlyOwn(t *testing.T) {
	gw := newDeviceScanTestGateway(t)
	h := NewDeviceScansHandler()

	mine := insertTestDeviceScan(t, gw, "user-mine", "device-mine")
	insertTestDeviceScan(t, gw, "user-other", "device-other")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/devices/scans?group_by_device=false", nil)
	if err := h.List(api.Context{
		ResponseWriter: rec,
		Request:        req,
		GatewayClient:  gw,
		User:           basicUser("user-mine"),
	}); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp apitypes.DeviceScanResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Total != 1 {
		t.Fatalf("total = %d, want 1", resp.Total)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(resp.Items))
	}
	if resp.Items[0].SubmittedBy != "user-mine" {
		t.Fatalf("items[0].SubmittedBy = %q, want %q", resp.Items[0].SubmittedBy, "user-mine")
	}
	if resp.Items[0].ID != mine.ID {
		t.Fatalf("items[0].ID = %d, want %d", resp.Items[0].ID, mine.ID)
	}
}

func TestDeviceScansList_BasicUserCannotWidenViaQueryParam(t *testing.T) {
	gw := newDeviceScanTestGateway(t)
	h := NewDeviceScansHandler()

	insertTestDeviceScan(t, gw, "user-mine", "device-a")
	insertTestDeviceScan(t, gw, "user-other", "device-b")
	insertTestDeviceScan(t, gw, "user-other", "device-c")

	// Try every shape that the multi-value parser accepts: repeated
	// params, comma-separated values, and a query that mixes our own
	// UID with somebody else's.
	for _, qs := range []string{
		"submitted_by=user-other",
		"submitted_by=user-other&submitted_by=user-mine",
		"submitted_by=user-other,user-mine",
	} {
		t.Run(qs, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/devices/scans?group_by_device=false&"+qs, nil)
			if err := h.List(api.Context{
				ResponseWriter: rec,
				Request:        req,
				GatewayClient:  gw,
				User:           basicUser("user-mine"),
			}); err != nil {
				t.Fatalf("List() error = %v", err)
			}

			var resp apitypes.DeviceScanResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Total != 1 {
				t.Fatalf("total = %d, want 1 (filter must not widen scope)", resp.Total)
			}
			for _, item := range resp.Items {
				if item.SubmittedBy != "user-mine" {
					t.Fatalf("leaked scan SubmittedBy = %q", item.SubmittedBy)
				}
			}
		})
	}
}

func TestDeviceScansList_AdminSeesAll(t *testing.T) {
	gw := newDeviceScanTestGateway(t)
	h := NewDeviceScansHandler()

	insertTestDeviceScan(t, gw, "user-mine", "device-a")
	insertTestDeviceScan(t, gw, "user-other", "device-b")
	insertTestDeviceScan(t, gw, "user-third", "device-c")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/devices/scans?group_by_device=false", nil)
	if err := h.List(api.Context{
		ResponseWriter: rec,
		Request:        req,
		GatewayClient:  gw,
		User:           adminUser("admin-user"),
	}); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	var resp apitypes.DeviceScanResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Total != 3 {
		t.Fatalf("total = %d, want 3", resp.Total)
	}
}

func TestDeviceScansList_AdminFilterHonored(t *testing.T) {
	gw := newDeviceScanTestGateway(t)
	h := NewDeviceScansHandler()

	insertTestDeviceScan(t, gw, "user-mine", "device-a")
	insertTestDeviceScan(t, gw, "user-other", "device-b")
	insertTestDeviceScan(t, gw, "user-other", "device-c")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/devices/scans?group_by_device=false&submitted_by=user-other", nil)
	if err := h.List(api.Context{
		ResponseWriter: rec,
		Request:        req,
		GatewayClient:  gw,
		User:           adminUser("admin-user"),
	}); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	var resp apitypes.DeviceScanResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Total != 2 {
		t.Fatalf("total = %d, want 2", resp.Total)
	}
	for _, item := range resp.Items {
		if item.SubmittedBy != "user-other" {
			t.Fatalf("filter dropped: SubmittedBy = %q", item.SubmittedBy)
		}
	}
}

func TestDeviceScansList_AuditorSeesAll(t *testing.T) {
	gw := newDeviceScanTestGateway(t)
	h := NewDeviceScansHandler()

	insertTestDeviceScan(t, gw, "user-mine", "device-a")
	insertTestDeviceScan(t, gw, "user-other", "device-b")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/devices/scans?group_by_device=false", nil)
	if err := h.List(api.Context{
		ResponseWriter: rec,
		Request:        req,
		GatewayClient:  gw,
		User:           auditorUser("auditor-user"),
	}); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	var resp apitypes.DeviceScanResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Total != 2 {
		t.Fatalf("total = %d, want 2", resp.Total)
	}
}

func TestDeviceScansGet_BasicUserOwnScanReturnsOK(t *testing.T) {
	gw := newDeviceScanTestGateway(t)
	h := NewDeviceScansHandler()

	mine := insertTestDeviceScan(t, gw, "user-mine", "device-a")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/devices/scans/"+strconv.FormatUint(uint64(mine.ID), 10), nil)
	req.SetPathValue("scan_id", strconv.FormatUint(uint64(mine.ID), 10))
	if err := h.Get(api.Context{
		ResponseWriter: rec,
		Request:        req,
		GatewayClient:  gw,
		User:           basicUser("user-mine"),
	}); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var got apitypes.DeviceScan
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != mine.ID {
		t.Fatalf("ID = %d, want %d", got.ID, mine.ID)
	}
	if got.SubmittedBy != "user-mine" {
		t.Fatalf("SubmittedBy = %q, want %q", got.SubmittedBy, "user-mine")
	}
}

func TestDeviceScansGet_BasicUserOtherUserScanIsIndistinguishableFromMissing(t *testing.T) {
	gw := newDeviceScanTestGateway(t)
	h := NewDeviceScansHandler()

	other := insertTestDeviceScan(t, gw, "user-other", "device-b")
	missingID := other.ID + 1000 // an ID that does not exist

	get := func(id uint) error {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/devices/scans/"+strconv.FormatUint(uint64(id), 10), nil)
		req.SetPathValue("scan_id", strconv.FormatUint(uint64(id), 10))
		return h.Get(api.Context{
			ResponseWriter: rec,
			Request:        req,
			GatewayClient:  gw,
			User:           basicUser("user-mine"),
		})
	}

	otherErr := get(other.ID)
	missingErr := get(missingID)

	for label, err := range map[string]error{"other-user-scan": otherErr, "missing-id": missingErr} {
		if err == nil {
			t.Fatalf("%s: expected error, got nil", label)
		}
		if !apitypes.IsNotFound(err) {
			t.Fatalf("%s: expected NotFound, got %v", label, err)
		}
		var httpErr *apitypes.ErrHTTP
		if !errors.As(err, &httpErr) {
			t.Fatalf("%s: expected *ErrHTTP, got %T", label, err)
		}
		if httpErr.Code != http.StatusNotFound {
			t.Fatalf("%s: code = %d, want 404", label, httpErr.Code)
		}
	}

	// Crucially, the two responses must be indistinguishable: the
	// message format ("device scan N not found") must match the actual
	// requested ID in both cases, with no hint that the "other" ID
	// actually exists in the DB.
	var otherHTTP, missingHTTP *apitypes.ErrHTTP
	if !errors.As(otherErr, &otherHTTP) || !errors.As(missingErr, &missingHTTP) {
		t.Fatalf("both errors must unwrap to *ErrHTTP")
	}
	wantOther := "device scan " + strconv.FormatUint(uint64(other.ID), 10) + " not found"
	wantMissing := "device scan " + strconv.FormatUint(uint64(missingID), 10) + " not found"
	if otherHTTP.Message != wantOther {
		t.Fatalf("other-user-scan message = %q, want %q", otherHTTP.Message, wantOther)
	}
	if missingHTTP.Message != wantMissing {
		t.Fatalf("missing-id message = %q, want %q", missingHTTP.Message, wantMissing)
	}
}

func TestDeviceScansGet_AdminCanReadAnyScan(t *testing.T) {
	gw := newDeviceScanTestGateway(t)
	h := NewDeviceScansHandler()

	other := insertTestDeviceScan(t, gw, "user-other", "device-b")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/devices/scans/"+strconv.FormatUint(uint64(other.ID), 10), nil)
	req.SetPathValue("scan_id", strconv.FormatUint(uint64(other.ID), 10))
	if err := h.Get(api.Context{
		ResponseWriter: rec,
		Request:        req,
		GatewayClient:  gw,
		User:           adminUser("admin-user"),
	}); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var got apitypes.DeviceScan
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.SubmittedBy != "user-other" {
		t.Fatalf("SubmittedBy = %q, want %q", got.SubmittedBy, "user-other")
	}
}
