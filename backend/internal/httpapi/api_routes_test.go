package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oleg3190/Web-studio-img/backend/internal/assetlibrary"
	"github.com/oleg3190/Web-studio-img/backend/internal/assets"
	"github.com/oleg3190/Web-studio-img/backend/internal/assistant"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/branches"
	"github.com/oleg3190/Web-studio-img/backend/internal/brief"
	"github.com/oleg3190/Web-studio-img/backend/internal/cardbatch"
	"github.com/oleg3190/Web-studio-img/backend/internal/cardtypes"
	"github.com/oleg3190/Web-studio-img/backend/internal/composition"
	"github.com/oleg3190/Web-studio-img/backend/internal/dna"
	"github.com/oleg3190/Web-studio-img/backend/internal/events"
	"github.com/oleg3190/Web-studio-img/backend/internal/exports"
	"github.com/oleg3190/Web-studio-img/backend/internal/generation"
	"github.com/oleg3190/Web-studio-img/backend/internal/humanactions"
	"github.com/oleg3190/Web-studio-img/backend/internal/iterations"
	"github.com/oleg3190/Web-studio-img/backend/internal/layers"
	"github.com/oleg3190/Web-studio-img/backend/internal/library"
	"github.com/oleg3190/Web-studio-img/backend/internal/manualedits"
	"github.com/oleg3190/Web-studio-img/backend/internal/printprofiles"
	"github.com/oleg3190/Web-studio-img/backend/internal/projects"
	"github.com/oleg3190/Web-studio-img/backend/internal/prompts"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
	"github.com/oleg3190/Web-studio-img/backend/internal/recipes"
	"github.com/oleg3190/Web-studio-img/backend/internal/references"
	"github.com/oleg3190/Web-studio-img/backend/internal/rights"
	"github.com/oleg3190/Web-studio-img/backend/internal/similarity"
	"github.com/oleg3190/Web-studio-img/backend/internal/styles"
	"github.com/oleg3190/Web-studio-img/backend/internal/templates"
	"github.com/oleg3190/Web-studio-img/backend/internal/variants"
	"github.com/oleg3190/Web-studio-img/backend/internal/visualdna"
	"github.com/oleg3190/Web-studio-img/backend/internal/workflow"
)

type routeCase struct {
	name   string
	method string
	path   string
	status int
	body   string
}

func TestAllAPIRoutesAreRegisteredAndReachTheirHandler(t *testing.T) {
	mux := http.NewServeMux()

	// Register the same concrete handler types used by cmd/api. Zero-value
	// handlers are intentional here: every protected endpoint must reject an
	// unauthenticated request before touching its backing store.
	(&assetlibrary.Handler{}).Register(mux)
	(&assets.UploadHandler{}).Register(mux)
	(&assistant.Handler{}).Register(mux)
	(&auth.HTTPHandler{}).Register(mux)
	(&branches.Handler{}).Register(mux)
	(&brief.Handler{}).Register(mux)
	(&cardbatch.Handler{}).Register(mux)
	(&cardtypes.Handler{}).Register(mux)
	(&composition.Handler{}).Register(mux)
	(&dna.Handler{}).Register(mux)
	(&events.Handler{}).Register(mux)
	(&exports.Handler{}).Register(mux)
	(&generation.Handler{}).Register(mux)
	(&humanactions.Handler{}).Register(mux)
	(&iterations.Handler{}).Register(mux)
	(&layers.Handler{}).Register(mux)
	(&library.Handler{}).Register(mux)
	(&manualedits.Handler{}).Register(mux)
	(&printprofiles.Handler{}).Register(mux)
	(&projects.Handler{}).Register(mux)
	(&prompts.Handler{}).Register(mux)
	(&provenance.Handler{}).Register(mux)
	(&recipes.Handler{}).Register(mux)
	(&references.Handler{}).Register(mux)
	(&rights.Handler{}).Register(mux)
	(&similarity.Handler{}).Register(mux)
	(&styles.Handler{}).Register(mux)
	(&templates.Handler{}).Register(mux)
	(&variants.Handler{}).Register(mux)
	(&visualdna.Handler{}).Register(mux)
	(&workflow.Handler{}).Register(mux)

	validUUID := "11111111-1111-1111-1111-111111111111"
	validUUID2 := "22222222-2222-2222-2222-222222222222"
	routes := []routeCase{
		// Authentication routes are public, so malformed payloads must stop
		// before a nil/invalid service can be dereferenced.
		{"auth-register", http.MethodPost, "/api/v1/auth/register", http.StatusBadRequest, "{}"},
		{"auth-login", http.MethodPost, "/api/v1/auth/login", http.StatusBadRequest, "{"},
		{"auth-me", http.MethodGet, "/api/v1/auth/me", http.StatusUnauthorized, ""},
		{"auth-logout", http.MethodPost, "/api/v1/auth/logout", http.StatusUnauthorized, ""},

		{"projects-list", http.MethodGet, "/api/v1/projects", http.StatusUnauthorized, ""},
		{"projects-create", http.MethodPost, "/api/v1/projects", http.StatusUnauthorized, "{}"},
		{"projects-get", http.MethodGet, "/api/v1/projects/"+validUUID, http.StatusUnauthorized, ""},
		{"projects-update", http.MethodPatch, "/api/v1/projects/"+validUUID, http.StatusUnauthorized, "{}"},
		{"projects-delete", http.MethodDelete, "/api/v1/projects/"+validUUID, http.StatusUnauthorized, ""},

		{"iterations-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/iterations", http.StatusUnauthorized, ""},
		{"iterations-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/iterations", http.StatusUnauthorized, "{}"},
		{"iterations-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/iterations/"+validUUID2, http.StatusUnauthorized, ""},
		{"iterations-global-get", http.MethodGet, "/api/v1/iterations/"+validUUID2, http.StatusUnauthorized, ""},
		{"iterations-restore", http.MethodPost, "/api/v1/projects/"+validUUID+"/iterations/"+validUUID2+"/restore", http.StatusUnauthorized, "{}"},

		{"generations-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/generations", http.StatusUnauthorized, "{}"},
		{"generations-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/generations/"+validUUID2, http.StatusUnauthorized, ""},
		{"generations-global-get", http.MethodGet, "/api/v1/generations/"+validUUID2, http.StatusUnauthorized, ""},
		{"generations-cancel-global", http.MethodPost, "/api/v1/generations/"+validUUID2+"/cancel", http.StatusUnauthorized, ""},
		{"generations-cancel", http.MethodPost, "/api/v1/projects/"+validUUID+"/generations/"+validUUID2+"/cancel", http.StatusUnauthorized, ""},
		{"generations-events", http.MethodGet, "/api/v1/projects/"+validUUID+"/generations/"+validUUID2+"/events", http.StatusUnauthorized, ""},

		{"branches-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/branches", http.StatusUnauthorized, ""},
		{"branches-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/branches", http.StatusUnauthorized, "{}"},
		{"branches-update", http.MethodPatch, "/api/v1/projects/"+validUUID+"/branches/"+validUUID2, http.StatusUnauthorized, "{}"},
		{"branches-compare", http.MethodGet, "/api/v1/projects/"+validUUID+"/branches/compare?source_branch_id="+validUUID+"&target_branch_id="+validUUID2, http.StatusUnauthorized, ""},
		{"branches-merge", http.MethodPost, "/api/v1/projects/"+validUUID+"/branches/"+validUUID2+"/merge", http.StatusUnauthorized, "{}"},

		{"events-stream", http.MethodGet, "/api/v1/projects/"+validUUID+"/events", http.StatusUnauthorized, ""},

		{"prompts-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/prompts", http.StatusUnauthorized, ""},
		{"prompts-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/prompts", http.StatusUnauthorized, "{}"},
		{"prompts-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/prompts/"+validUUID2, http.StatusUnauthorized, ""},
		{"prompts-patch", http.MethodPatch, "/api/v1/projects/"+validUUID+"/prompts/"+validUUID2, http.StatusUnauthorized, "{}"},
		{"prompts-global-patch", http.MethodPatch, "/api/v1/prompts/"+validUUID2, http.StatusUnauthorized, "{}"},
		{"prompts-approve", http.MethodPost, "/api/v1/projects/"+validUUID+"/prompts/"+validUUID2+"/approve", http.StatusUnauthorized, "{}"},

		{"references-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/references", http.StatusUnauthorized, ""},
		{"references-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/references", http.StatusUnauthorized, "{}"},
		{"references-update", http.MethodPatch, "/api/v1/projects/"+validUUID+"/references/"+validUUID2, http.StatusUnauthorized, "{}"},
		{"references-create-usage", http.MethodPost, "/api/v1/projects/"+validUUID+"/references/"+validUUID2+"/usage", http.StatusUnauthorized, "{}"},
		{"references-list-usage", http.MethodGet, "/api/v1/projects/"+validUUID+"/references/"+validUUID2+"/usage", http.StatusUnauthorized, ""},
		{"references-delete", http.MethodDelete, "/api/v1/projects/"+validUUID+"/references/"+validUUID2, http.StatusUnauthorized, ""},
		{"references-global-delete", http.MethodDelete, "/api/v1/references/"+validUUID2, http.StatusUnauthorized, ""},

		{"human-actions-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/human-actions", http.StatusUnauthorized, ""},
		{"human-actions-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/human-actions", http.StatusUnauthorized, "{}"},

		{"provenance-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/provenance", http.StatusUnauthorized, ""},
		{"provenance-verify", http.MethodGet, "/api/v1/projects/"+validUUID+"/provenance/verify", http.StatusUnauthorized, ""},

		{"assets-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/assets", http.StatusUnauthorized, ""},
		{"assets-import-url", http.MethodPost, "/api/v1/projects/"+validUUID+"/assets/import-url", http.StatusUnauthorized, "{}"},
		{"assets-init-upload", http.MethodPost, "/api/v1/projects/"+validUUID+"/assets/uploads", http.StatusUnauthorized, "{}"},
		{"assets-part", http.MethodPut, "/api/v1/projects/"+validUUID+"/assets/"+validUUID2+"/parts/0", http.StatusUnauthorized, ""},
		{"assets-complete", http.MethodPost, "/api/v1/projects/"+validUUID+"/assets/"+validUUID2+"/complete", http.StatusUnauthorized, ""},
		{"assets-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/assets/"+validUUID2, http.StatusUnauthorized, ""},
		{"assets-download-url", http.MethodGet, "/api/v1/projects/"+validUUID+"/assets/"+validUUID2+"/download-url", http.StatusUnauthorized, ""},
		{"storage-local-put", http.MethodPut, "/api/v1/storage/local", http.StatusUnauthorized, ""},
		{"storage-local-get", http.MethodGet, "/api/v1/storage/local", http.StatusUnauthorized, ""},

		{"similarity-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/similarity-checks", http.StatusUnauthorized, "{}"},
		{"similarity-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/similarity-checks/"+validUUID2, http.StatusUnauthorized, ""},
		{"similarity-global-get", http.MethodGet, "/api/v1/similarity-checks/"+validUUID2, http.StatusUnauthorized, ""},
		{"similarity-influence", http.MethodPost, "/api/v1/projects/"+validUUID+"/references/"+validUUID2+"/influence", http.StatusUnauthorized, "{}"},

		{"exports-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/exports", http.StatusUnauthorized, "{}"},
		{"exports-project-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/exports/"+validUUID2, http.StatusUnauthorized, ""},
		{"exports-global-get", http.MethodGet, "/api/v1/exports/"+validUUID2, http.StatusUnauthorized, ""},
		{"exports-legacy", http.MethodPost, "/api/v1/projects/"+validUUID+"/assets/"+validUUID2+"/export", http.StatusUnauthorized, "{}"},

		{"assistant-tools", http.MethodGet, "/api/v1/projects/"+validUUID+"/assistant/tools", http.StatusUnauthorized, ""},
		{"assistant-actions", http.MethodGet, "/api/v1/projects/"+validUUID+"/assistant/actions", http.StatusUnauthorized, ""},
		{"assistant-tool-execute", http.MethodPost, "/api/v1/projects/"+validUUID+"/assistant/tools/get_project_history", http.StatusUnauthorized, "{}"},
		{"assistant-decision", http.MethodPost, "/api/v1/projects/"+validUUID+"/assistant/recommendations/"+validUUID2+"/decision", http.StatusUnauthorized, "{}"},

		{"composition-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/iterations/"+validUUID2+"/composition", http.StatusUnauthorized, ""},
		{"composition-put", http.MethodPut, "/api/v1/projects/"+validUUID+"/iterations/"+validUUID2+"/composition", http.StatusUnauthorized, "{}"},
		{"composition-suggest", http.MethodPost, "/api/v1/projects/"+validUUID+"/composition-mutation-suggestions", http.StatusUnauthorized, "{}"},
		{"composition-accept", http.MethodPost, "/api/v1/projects/"+validUUID+"/composition-mutation-suggestions/"+validUUID2+"/accept", http.StatusUnauthorized, "{}"},
		{"composition-reject", http.MethodPost, "/api/v1/projects/"+validUUID+"/composition-mutation-suggestions/"+validUUID2+"/reject", http.StatusUnauthorized, "{}"},

		{"materials-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/materials", http.StatusUnauthorized, ""},
		{"materials-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/materials", http.StatusUnauthorized, "{}"},
		{"materials-update", http.MethodPatch, "/api/v1/projects/"+validUUID+"/materials/"+validUUID2, http.StatusUnauthorized, "{}"},
		{"materials-delete", http.MethodDelete, "/api/v1/projects/"+validUUID+"/materials/"+validUUID2, http.StatusUnauthorized, ""},
		{"materials-select", http.MethodPost, "/api/v1/projects/"+validUUID+"/materials/"+validUUID2+"/select", http.StatusUnauthorized, ""},
		{"textures-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/textures", http.StatusUnauthorized, ""},
		{"textures-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/textures", http.StatusUnauthorized, "{}"},
		{"textures-update", http.MethodPatch, "/api/v1/projects/"+validUUID+"/textures/"+validUUID2, http.StatusUnauthorized, "{}"},
		{"textures-delete", http.MethodDelete, "/api/v1/projects/"+validUUID+"/textures/"+validUUID2, http.StatusUnauthorized, ""},
		{"textures-select", http.MethodPost, "/api/v1/projects/"+validUUID+"/textures/"+validUUID2+"/select", http.StatusUnauthorized, ""},

		{"style-global-list", http.MethodGet, "/api/v1/style-profiles", http.StatusUnauthorized, ""},
		{"style-global-create", http.MethodPost, "/api/v1/style-profiles", http.StatusUnauthorized, "{}"},
		{"style-global-get", http.MethodGet, "/api/v1/style-profiles/"+validUUID2, http.StatusUnauthorized, ""},
		{"style-global-update", http.MethodPatch, "/api/v1/style-profiles/"+validUUID2, http.StatusUnauthorized, "{}"},
		{"style-project-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/style-profiles", http.StatusUnauthorized, ""},
		{"style-infer", http.MethodPost, "/api/v1/projects/"+validUUID+"/style-profiles/infer", http.StatusUnauthorized, "{}"},
		{"style-apply", http.MethodPost, "/api/v1/projects/"+validUUID+"/style-profiles/"+validUUID2+"/apply", http.StatusUnauthorized, "{}"},

		{"asset-dna-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/asset-dna", http.StatusUnauthorized, ""},
		{"asset-dna-analyze", http.MethodPost, "/api/v1/projects/"+validUUID+"/asset-dna/analyze", http.StatusUnauthorized, "{}"},
		{"visual-language-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/visual-language", http.StatusUnauthorized, ""},
		{"visual-language-analyze", http.MethodPost, "/api/v1/projects/"+validUUID+"/visual-language/analyze", http.StatusUnauthorized, "{}"},
		{"visual-language-apply", http.MethodPost, "/api/v1/projects/"+validUUID+"/visual-language/"+validUUID2+"/apply", http.StatusUnauthorized, "{}"},

		{"rights-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/rights", http.StatusUnauthorized, ""},
		{"rights-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/rights", http.StatusUnauthorized, "{}"},
		{"rights-update", http.MethodPatch, "/api/v1/projects/"+validUUID+"/rights/"+validUUID2, http.StatusUnauthorized, "{}"},
		{"rights-delete", http.MethodDelete, "/api/v1/projects/"+validUUID+"/rights/"+validUUID2, http.StatusUnauthorized, ""},
		{"constraints-global-list", http.MethodGet, "/api/v1/constraints", http.StatusUnauthorized, ""},
		{"constraints-global-create", http.MethodPost, "/api/v1/constraints", http.StatusUnauthorized, "{}"},
		{"constraints-project-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/constraints", http.StatusUnauthorized, ""},
		{"constraints-project-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/constraints", http.StatusUnauthorized, "{}"},
		{"constraints-update", http.MethodPatch, "/api/v1/projects/"+validUUID+"/constraints/"+validUUID2, http.StatusUnauthorized, "{}"},
		{"constraints-delete", http.MethodDelete, "/api/v1/projects/"+validUUID+"/constraints/"+validUUID2, http.StatusUnauthorized, ""},

		{"layers-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/layers", http.StatusUnauthorized, ""},
		{"layers-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/layers", http.StatusUnauthorized, "{}"},
		{"layers-update", http.MethodPatch, "/api/v1/projects/"+validUUID+"/layers/"+validUUID2, http.StatusUnauthorized, "{}"},
		{"layers-delete", http.MethodDelete, "/api/v1/projects/"+validUUID+"/layers/"+validUUID2, http.StatusUnauthorized, ""},

		{"manual-edits-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/manual-edits", http.StatusUnauthorized, "{}"},
		{"manual-edits-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/manual-edits", http.StatusUnauthorized, ""},
		{"manual-edits-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/manual-edits/"+validUUID2, http.StatusUnauthorized, ""},
		{"manual-edits-apply", http.MethodPost, "/api/v1/projects/"+validUUID+"/manual-edits/"+validUUID2+"/apply", http.StatusUnauthorized, "{}"},
		{"manual-edits-reject", http.MethodPost, "/api/v1/projects/"+validUUID+"/manual-edits/"+validUUID2+"/reject", http.StatusUnauthorized, ""},

		{"variants-sources", http.MethodGet, "/api/v1/projects/"+validUUID+"/variant-sources", http.StatusUnauthorized, ""},
		{"variants-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/variant-sets", http.StatusUnauthorized, ""},
		{"variants-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/variant-sets", http.StatusUnauthorized, "{}"},
		{"variants-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/variant-sets/"+validUUID2, http.StatusUnauthorized, ""},
		{"variants-compare", http.MethodGet, "/api/v1/projects/"+validUUID+"/variant-sets/"+validUUID2+"/compare", http.StatusUnauthorized, ""},
		{"variants-patch", http.MethodPatch, "/api/v1/projects/"+validUUID+"/variant-sets/"+validUUID2+"/variants/"+validUUID, http.StatusUnauthorized, "{}"},
		{"variants-regenerate", http.MethodPost, "/api/v1/projects/"+validUUID+"/variant-sets/"+validUUID2+"/variants/"+validUUID+"/regenerate", http.StatusUnauthorized, "{}"},
		{"variants-create-iteration", http.MethodPost, "/api/v1/projects/"+validUUID+"/variant-sets/"+validUUID2+"/iterations", http.StatusUnauthorized, "{}"},
		{"variants-rejection-summary", http.MethodGet, "/api/v1/projects/"+validUUID+"/variant-rejection-summary", http.StatusUnauthorized, ""},
		{"variants-rejection-timeline", http.MethodGet, "/api/v1/projects/"+validUUID+"/variant-rejection-timeline", http.StatusUnauthorized, ""},

		{"brief-current", http.MethodGet, "/api/v1/projects/"+validUUID+"/brief", http.StatusUnauthorized, ""},
		{"brief-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/brief/versions", http.StatusUnauthorized, ""},
		{"brief-approved", http.MethodGet, "/api/v1/projects/"+validUUID+"/brief/approved", http.StatusUnauthorized, ""},
		{"brief-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/brief", http.StatusUnauthorized, "{}"},
		{"brief-approve", http.MethodPost, "/api/v1/projects/"+validUUID+"/brief/"+validUUID2+"/approve", http.StatusUnauthorized, "{}"},

		{"card-batches-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/card-batches", http.StatusUnauthorized, ""},
		{"card-batches-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/card-batches", http.StatusUnauthorized, "{}"},
		{"card-batches-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/card-batches/"+validUUID2, http.StatusUnauthorized, ""},
		{"card-batches-update", http.MethodPatch, "/api/v1/projects/"+validUUID+"/card-batches/"+validUUID2, http.StatusUnauthorized, "{}"},
		{"card-batches-delete", http.MethodDelete, "/api/v1/projects/"+validUUID+"/card-batches/"+validUUID2, http.StatusUnauthorized, ""},

		{"card-types-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/card-types", http.StatusUnauthorized, ""},
		{"card-types-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/card-types", http.StatusUnauthorized, "{}"},
		{"card-types-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/card-types/"+validUUID2, http.StatusUnauthorized, ""},
		{"card-types-versions", http.MethodGet, "/api/v1/projects/"+validUUID+"/card-types/"+validUUID2+"/versions", http.StatusUnauthorized, ""},
		{"card-types-clone", http.MethodPost, "/api/v1/projects/"+validUUID+"/card-types/"+validUUID2+"/clone", http.StatusUnauthorized, "{}"},
		{"card-types-archive", http.MethodPost, "/api/v1/projects/"+validUUID+"/card-types/"+validUUID2+"/archive", http.StatusUnauthorized, "{}"},
		{"card-types-update", http.MethodPatch, "/api/v1/projects/"+validUUID+"/card-types/"+validUUID2, http.StatusUnauthorized, "{}"},

		{"print-profiles-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/print-profiles", http.StatusUnauthorized, ""},
		{"print-profiles-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/print-profiles", http.StatusUnauthorized, "{}"},
		{"print-profiles-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/print-profiles/"+validUUID2, http.StatusUnauthorized, ""},
		{"print-profiles-versions", http.MethodGet, "/api/v1/projects/"+validUUID+"/print-profiles/"+validUUID2+"/versions", http.StatusUnauthorized, ""},
		{"print-profiles-update", http.MethodPatch, "/api/v1/projects/"+validUUID+"/print-profiles/"+validUUID2, http.StatusUnauthorized, "{}"},
		{"print-profiles-archive", http.MethodPost, "/api/v1/projects/"+validUUID+"/print-profiles/"+validUUID2+"/archive", http.StatusUnauthorized, "{}"},

		{"templates-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/templates", http.StatusUnauthorized, ""},
		{"templates-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/templates", http.StatusUnauthorized, "{}"},
		{"templates-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/templates/"+validUUID2, http.StatusUnauthorized, ""},
		{"templates-versions", http.MethodGet, "/api/v1/projects/"+validUUID+"/templates/"+validUUID2+"/versions", http.StatusUnauthorized, ""},
		{"templates-update", http.MethodPatch, "/api/v1/projects/"+validUUID+"/templates/"+validUUID2, http.StatusUnauthorized, "{}"},
		{"templates-archive", http.MethodPost, "/api/v1/projects/"+validUUID+"/templates/"+validUUID2+"/archive", http.StatusUnauthorized, ""},

		{"recipes-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/recipes", http.StatusUnauthorized, ""},
		{"recipes-create", http.MethodPost, "/api/v1/projects/"+validUUID+"/recipes", http.StatusUnauthorized, "{}"},
		{"recipes-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/recipes/"+validUUID2, http.StatusUnauthorized, ""},
		{"recipes-version", http.MethodPost, "/api/v1/projects/"+validUUID+"/recipes/"+validUUID2+"/versions", http.StatusUnauthorized, "{}"},
		{"recipes-publish", http.MethodPost, "/api/v1/projects/"+validUUID+"/recipes/"+validUUID2+"/publish", http.StatusUnauthorized, ""},
		{"recipes-unpublish", http.MethodPost, "/api/v1/projects/"+validUUID+"/recipes/"+validUUID2+"/unpublish", http.StatusUnauthorized, ""},
		{"recipes-duplicate", http.MethodPost, "/api/v1/projects/"+validUUID+"/recipes/"+validUUID2+"/duplicate", http.StatusUnauthorized, ""},
		{"recipes-compatibility", http.MethodPost, "/api/v1/projects/"+validUUID+"/recipes/"+validUUID2+"/compatibility", http.StatusUnauthorized, "{}"},
		{"recipes-from-generation", http.MethodPost, "/api/v1/projects/"+validUUID+"/recipes/from-generation/"+validUUID2+"/create", http.StatusUnauthorized, "{}"},

		{"asset-library-list", http.MethodGet, "/api/v1/library/assets", http.StatusUnauthorized, ""},
		{"asset-library-create", http.MethodPost, "/api/v1/library/assets", http.StatusUnauthorized, "{}"},
		{"asset-library-get", http.MethodGet, "/api/v1/library/assets/"+validUUID, http.StatusUnauthorized, ""},
		{"asset-library-update", http.MethodPatch, "/api/v1/library/assets/"+validUUID, http.StatusUnauthorized, "{}"},
		{"asset-library-delete", http.MethodDelete, "/api/v1/library/assets/"+validUUID, http.StatusUnauthorized, ""},
		{"asset-library-versions", http.MethodGet, "/api/v1/library/assets/"+validUUID+"/versions", http.StatusUnauthorized, ""},
		{"asset-library-version-create", http.MethodPost, "/api/v1/library/assets/"+validUUID+"/versions", http.StatusUnauthorized, "{}"},
		{"asset-library-usage", http.MethodGet, "/api/v1/library/assets/"+validUUID+"/usage", http.StatusUnauthorized, ""},
		{"asset-library-use", http.MethodPost, "/api/v1/library/assets/"+validUUID+"/use", http.StatusUnauthorized, "{}"},
		{"asset-library-sources", http.MethodGet, "/api/v1/library/sources", http.StatusUnauthorized, ""},

		{"workflow-mode-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/mode", http.StatusUnauthorized, ""},
		{"workflow-mode-set", http.MethodPut, "/api/v1/projects/"+validUUID+"/mode", http.StatusUnauthorized, "{}"},
		{"workflow-lifecycle-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/lifecycle", http.StatusUnauthorized, ""},
		{"workflow-lifecycle", http.MethodPost, "/api/v1/projects/"+validUUID+"/lifecycle", http.StatusUnauthorized, "{}"},
		{"workflow-asset-lifecycle", http.MethodPost, "/api/v1/projects/"+validUUID+"/assets/"+validUUID2+"/lifecycle", http.StatusUnauthorized, "{}"},
		{"workflow-usage", http.MethodGet, "/api/v1/projects/"+validUUID+"/usage", http.StatusUnauthorized, ""},
		{"workflow-charge", http.MethodPost, "/api/v1/projects/"+validUUID+"/charges", http.StatusUnauthorized, "{}"},
		{"workflow-search", http.MethodGet, "/api/v1/search?q=test", http.StatusUnauthorized, ""},

		{"visual-dna-list", http.MethodGet, "/api/v1/projects/"+validUUID+"/visual-dna", http.StatusUnauthorized, ""},
		{"visual-dna-sources", http.MethodGet, "/api/v1/projects/"+validUUID+"/visual-dna/sources", http.StatusUnauthorized, ""},
		{"visual-dna-analyze", http.MethodPost, "/api/v1/projects/"+validUUID+"/visual-dna/analyze", http.StatusUnauthorized, "{}"},
		{"visual-dna-get", http.MethodGet, "/api/v1/projects/"+validUUID+"/visual-dna/profiles/"+validUUID2, http.StatusUnauthorized, ""},
		{"visual-dna-recompute", http.MethodPost, "/api/v1/projects/"+validUUID+"/visual-dna/profiles/"+validUUID2+"/recompute", http.StatusUnauthorized, ""},
		{"visual-dna-compare", http.MethodGet, "/api/v1/projects/"+validUUID+"/visual-dna/profiles/"+validUUID2+"/compare", http.StatusUnauthorized, ""},
		{"visual-dna-suggestion", http.MethodGet, "/api/v1/projects/"+validUUID+"/visual-dna/profiles/"+validUUID2+"/suggestion", http.StatusUnauthorized, ""},
	}

	for _, tc := range routes {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()

			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s %s panicked: %v", tc.method, tc.path, r)
				}
			}()

			mux.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("%s %s: status=%d body=%s, want %d", tc.method, tc.path, rec.Code, rec.Body.String(), tc.status)
			}
		})
	}
}

func TestAllAPIRoutesRejectUnsupportedMethods(t *testing.T) {
	mux := http.NewServeMux()
	(&projects.Handler{}).Register(mux)
	(&iterations.Handler{}).Register(mux)
	(&generation.Handler{}).Register(mux)
	(&auth.HTTPHandler{}).Register(mux)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/api/v1/projects"},
		{http.MethodDelete, "/api/v1/auth/register"},
		{http.MethodPatch, "/api/v1/auth/login"},
		{http.MethodPut, "/api/v1/projects/11111111-1111-1111-1111-111111111111/iterations"},
		{http.MethodGet, "/api/v1/auth/register"},
		{http.MethodGet, "/api/v1/projects/11111111-1111-1111-1111-111111111111/generations"},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s %s: status=%d, want 405", tc.method, tc.path, rec.Code)
			}
		})
	}
}
