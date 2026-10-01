package assistant

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/oleg3190/Web-studio-img/backend/internal/assets"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/events"
	"github.com/oleg3190/Web-studio-img/backend/internal/generation"
	"github.com/oleg3190/Web-studio-img/backend/internal/iterations"
	"github.com/oleg3190/Web-studio-img/backend/internal/prompts"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
	"github.com/oleg3190/Web-studio-img/backend/internal/providers/comfyui"
	"github.com/oleg3190/Web-studio-img/backend/internal/references"
	"github.com/oleg3190/Web-studio-img/backend/internal/similarity"
	"github.com/oleg3190/Web-studio-img/backend/internal/storage"
	"github.com/oleg3190/Web-studio-img/backend/internal/library"
)

const (
	kindTool = "tool_execution"
	kindRecommendation = "recommendation"
)

type Handler struct {
	db *sql.DB
	iterations *iterations.SQLStore
	prompts *prompts.Store
	refs *references.Store
	assets *assets.Store
	storage storage.StorageProvider
	events *events.Store
	provenance *provenance.Store
	actions *Store
	queue generation.Enqueuer
	provider generation.Provider
	flowPlanner *FlowPlanner
}

type Config struct {
	DB *sql.DB
	Iterations *iterations.SQLStore
	Prompts *prompts.Store
	References *references.Store
	Assets *assets.Store
	Storage storage.StorageProvider
	Events *events.Store
	Provenance *provenance.Store
	Actions *Store
	Queue generation.Enqueuer
	Provider generation.Provider
}

type ToolDescriptor struct {
	Name string `json:"name"`
	Description string `json:"description"`
	Mutating bool `json:"mutating"`
	Recommendation bool `json:"recommendation"`
}

var toolCatalog = []ToolDescriptor{
	{Name:"inspect_comfyui_capabilities",Description:"Inspect the live ComfyUI runtime: nodes, models, version and devices."},
	{Name:"create_comfy_flow",Description:"Create a ComfyUI flow from a natural-language task using only live runtime capabilities."},
	{Name:"validate_comfy_flow",Description:"Validate a ComfyUI API workflow against the live installed node and model catalog."},
	{Name:"run_comfy_flow",Description:"Validate and queue a generated ComfyUI flow through the normal generation pipeline.",Mutating:true},
	{Name:"create_iteration",Description:"Create one immutable creative iteration.",Mutating:true},
	{Name:"get_project_history",Description:"Return the owned project's creative history."},
	{Name:"analyze_composition",Description:"Analyze an owned image with deterministic composition descriptors.",Recommendation:true},
	{Name:"search_references",Description:"Search only references available to the project."},
	{Name:"check_similarity",Description:"Check a target asset against project reference assets."},
	{Name:"suggest_prompt",Description:"Suggest a prompt refinement from the latest prompt.",Recommendation:true},
	{Name:"suggest_materials",Description:"Suggest project-visible material or texture library items.",Recommendation:true},
	{Name:"create_generation",Description:"Queue a generation through the configured provider.",Mutating:true},
	{Name:"compare_iterations",Description:"Compare two owned immutable iterations using prompts plus deterministic visual and composition descriptors."},
	{Name:"verify_provenance",Description:"Verify the project's persisted provenance hash chain."},
	{Name:"fact_check",Description:"Classify a factual claim from user-supplied evidence without inventing facts."},
}

func NewHandler(cfg Config) (*Handler,error) {
	if cfg.DB==nil||cfg.Iterations==nil||cfg.Actions==nil{return nil,errors.New("assistant requires database, iteration store and action store")}
	return &Handler{db:cfg.DB,iterations:cfg.Iterations,prompts:cfg.Prompts,refs:cfg.References,assets:cfg.Assets,storage:cfg.Storage,events:cfg.Events,provenance:cfg.Provenance,actions:cfg.Actions,queue:cfg.Queue,provider:cfg.Provider,flowPlanner:NewFlowPlannerFromEnv()},nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{project_id}/assistant/tools",h.tools)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/assistant/actions",h.listActions)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/assistant/tools/{tool}",h.execute)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/assistant/recommendations/{action_id}/decision",h.decide)
}

func (h *Handler) tools(w http.ResponseWriter,r *http.Request){
	p,ok:=auth.PrincipalFromContext(r.Context());if !ok{errJSON(w,401,"unauthorized","authentication required");return}
	var exists bool
	if err:=h.db.QueryRowContext(r.Context(),`SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')`,parsePathUUID(r,"project_id"),p.UserID).Scan(&exists);err!=nil||!exists{errJSON(w,404,"project_not_found","project not found");return}
	writeJSON(w,200,map[string]any{"tools":toolCatalog})
}

func (h *Handler) listActions(w http.ResponseWriter,r *http.Request){
	p,ok:=auth.PrincipalFromContext(r.Context());if !ok{errJSON(w,401,"unauthorized","authentication required");return}
	pid,e:=uuid.Parse(r.PathValue("project_id"));if e!=nil{errJSON(w,400,"invalid_project_id","invalid project id");return}
	items,e:=h.actions.List(r.Context(),p.UserID,pid);if e!=nil{errJSON(w,500,"assistant_actions_failed","could not list assistant actions");return}
	writeJSON(w,200,map[string]any{"actions":items})
}

func (h *Handler) execute(w http.ResponseWriter,r *http.Request){
	p,ok:=auth.PrincipalFromContext(r.Context());if !ok{errJSON(w,401,"unauthorized","authentication required");return}
	pid,e:=uuid.Parse(r.PathValue("project_id"));if e!=nil{errJSON(w,400,"invalid_project_id","invalid project id");return}
	if !h.ownedProject(r.Context(),p.UserID,pid){errJSON(w,404,"project_not_found","project not found");return}
	tool:=strings.TrimSpace(r.PathValue("tool"));d:=descriptor(tool);if d==nil{errJSON(w,404,"assistant_tool_not_found","tool is not allowlisted");return}
	input:=map[string]any{}
	if r.Body!=nil&&r.ContentLength!=0{if e:=decode(r,&input);e!=nil{errJSON(w,400,"invalid_request","invalid tool input");return}}
	key:=strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if d.Mutating&&key==""{errJSON(w,400,"missing_idempotency_key","Idempotency-Key header is required for mutating assistant tools");return}
	if key!=""{if existing,e:=h.actions.GetByIdempotencyKey(r.Context(),p.UserID,pid,key);e==nil{writeJSON(w,200,map[string]any{"action":existing,"result":existing.Output,"explanation":existing.Explanation,"confidence":existing.Confidence,"uncertainty":existing.Uncertainty});return}}
	var result map[string]any;var explanation,uncertainty string;var confidence float64;kind:=kindTool
	switch tool{
	case "create_iteration":result,explanation,confidence,uncertainty,e=h.createIteration(r.Context(),p.UserID,pid,input)
	case "get_project_history":result,explanation,confidence,uncertainty,e=h.history(r.Context(),p.UserID,pid)
	case "analyze_composition":result,explanation,confidence,uncertainty,e=h.analyzeComposition(r.Context(),p.UserID,pid,input);kind=kindRecommendation
	case "search_references":result,explanation,confidence,uncertainty,e=h.searchReferences(r.Context(),p.UserID,pid,input)
	case "check_similarity":result,explanation,confidence,uncertainty,e=h.checkSimilarity(r.Context(),p.UserID,pid,input)
	case "suggest_prompt":result,explanation,confidence,uncertainty,e=h.suggestPrompt(r.Context(),p.UserID,pid,input);kind=kindRecommendation
	case "suggest_materials":result,explanation,confidence,uncertainty,e=h.suggestMaterials(r.Context(),p.UserID,pid,input);kind=kindRecommendation
	case "create_generation":result,explanation,confidence,uncertainty,e=h.createGeneration(r.Context(),p.UserID,pid,input,key)
	case "compare_iterations":result,explanation,confidence,uncertainty,e=h.compareIterations(r.Context(),p.UserID,pid,input)
	case "verify_provenance":result,explanation,confidence,uncertainty,e=h.verifyProvenance(r.Context(),p.UserID,pid)
	case "inspect_comfyui_capabilities":result,explanation,confidence,uncertainty,e=h.inspectComfyUICapabilities(r.Context(),p.UserID,pid)
	case "create_comfy_flow":result,explanation,confidence,uncertainty,e=h.createComfyFlow(r.Context(),p.UserID,pid,input)
	case "validate_comfy_flow":result,explanation,confidence,uncertainty,e=h.validateComfyFlow(r.Context(),p.UserID,pid,input)
	case "run_comfy_flow":result,explanation,confidence,uncertainty,e=h.runComfyFlow(r.Context(),p.UserID,pid,input,key)
	case "fact_check":result,explanation,confidence,uncertainty,e=factCheck(input)
	default:e=errors.New("unsupported tool")
	}
	if e!=nil{errJSON(w,400,"assistant_tool_failed",e.Error());return}
	if kind==kindRecommendation {
		result = withRecommendation(tool, result, explanation, confidence, uncertainty)
	}
	action,e:=h.actions.Create(r.Context(),p.UserID,pid,tool,kind,input,result,explanation,confidence,uncertainty,key);if e!=nil{errJSON(w,500,"assistant_action_log_failed","could not record AI action");return}
	if h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:p.UserID,ProjectID:pid,EntityType:"assistant_action",EntityID:action.ID,Action:"assistant.tool.executed",Payload:map[string]any{"tool":tool,"kind":kind,"confidence":confidence}})}
	if h.events!=nil{_,_=h.events.Append(r.Context(),p.UserID,pid,"assistant.tool.completed","assistant_action",action.ID,map[string]any{"tool":tool,"kind":kind})}
	writeJSON(w,200,map[string]any{"action":action,"result":result,"explanation":explanation,"confidence":confidence,"uncertainty":uncertainty})
}


type comfyFlowRuntime interface {
	CapabilitySnapshot(context.Context) (comfyui.RuntimeCapabilitySnapshot,error)
	ValidateWorkflow(context.Context,map[string]any) (comfyui.WorkflowValidation,error)
}

func (h *Handler) comfyRuntime() (comfyFlowRuntime,error) {
	if h.provider==nil{return nil,errors.New("ComfyUI provider is not configured")}
	runtime,ok:=h.provider.(comfyFlowRuntime);if !ok{return nil,errors.New("active provider does not expose ComfyUI runtime capabilities")}
	return runtime,nil
}

func (h *Handler) inspectComfyUICapabilities(ctx context.Context,userID,projectID uuid.UUID)(map[string]any,string,float64,string,error) {
	if !h.ownedProject(ctx,userID,projectID){return nil,"",0,"",errors.New("project not found")}
	runtime,err:=h.comfyRuntime();if err!=nil{return nil,"",0,"",err}
	snapshot,err:=runtime.CapabilitySnapshot(ctx);if err!=nil{return nil,"",0,"",err}
	return map[string]any{"provider":"comfyui","runtime":compactRuntime("",snapshot),"retrieved_at":snapshot.RetrievedAt},"Live capabilities were read from the configured ComfyUI instance.",0.99,"Runtime data may change when models or custom nodes are installed.",nil
}

func (h *Handler) createComfyFlow(ctx context.Context,userID,projectID uuid.UUID,in map[string]any)(map[string]any,string,float64,string,error) {
	task,_:=in["task"].(string);task=strings.TrimSpace(task);if task==""{return nil,"",0,"",errors.New("task is required")}
	runtime,err:=h.comfyRuntime();if err!=nil{return nil,"",0,"",err}
	snapshot,err:=runtime.CapabilitySnapshot(ctx);if err!=nil{return nil,"",0,"",err}
	if reusable,ok,findErr:=h.findReusableRecipe(ctx,userID,projectID,task,runtime);findErr!=nil{return nil,"",0,"",findErr}else if ok{return reusable,"Reused an existing recipe after validating it against the live ComfyUI runtime.",0.95,"The match is heuristic; a new flow is generated when no compatible recipe matches.",nil}
	if h.flowPlanner==nil||!h.flowPlanner.Enabled(){return nil,"",0,"","flow planner is not configured; set FLOW_PLANNER_BASE_URL and FLOW_PLANNER_MODEL"}
	var validationErrors []string
	for attempt:=0;attempt<3;attempt++{
		plan,planErr:=h.flowPlanner.Plan(ctx,task,snapshot,validationErrors);if planErr!=nil{return nil,"",0,"",planErr}
		validation,validationErr:=runtime.ValidateWorkflow(ctx,plan.Workflow);if validationErr!=nil{return nil,"",0,"",validationErr}
		if validation.Compatible{
			result:=flowPlanMap(plan)
			result["source"]="ai_generated"
			result["compatibility"]=map[string]any{"compatible":true,"errors":validation.Errors,"warnings":validation.Warnings,"referenced_nodes":validation.ReferencedNodes,"referenced_models":validation.ReferencedModels}
			result["runtime_retrieved_at"]=snapshot.RetrievedAt
			return result,"AI generated a flow constrained to the live ComfyUI node and model catalog.",0.9,"The planner is retried with concrete runtime validation errors when needed.",nil
		}
		validationErrors=append([]string(nil),validation.Errors...)
	}
	return nil,"",0,"","AI could not produce a workflow compatible with the current ComfyUI runtime after three attempts"
}

func (h *Handler) validateComfyFlow(ctx context.Context,userID,projectID uuid.UUID,in map[string]any)(map[string]any,string,float64,string,error) {
	workflow,_:=in["workflow"].(map[string]any);if len(workflow)==0{return nil,"",0,"",errors.New("workflow is required")}
	runtime,err:=h.comfyRuntime();if err!=nil{return nil,"",0,"",err}
	validation,err:=runtime.ValidateWorkflow(ctx,workflow);if err!=nil{return nil,"",0,"",err}
	return map[string]any{"compatible":validation.Compatible,"errors":validation.Errors,"warnings":validation.Warnings,"referenced_nodes":validation.ReferencedNodes,"referenced_models":validation.ReferencedModels},"Validation used the live ComfyUI node and model catalog.",0.99,"This is runtime catalog and structural validation; queueing remains a separate step.",nil
}

func (h *Handler) runComfyFlow(ctx context.Context,userID,projectID uuid.UUID,in map[string]any,key string)(map[string]any,string,float64,string,error) {
	if h.queue==nil||h.provider==nil{return nil,"",0,"",errors.New("generation provider is not configured")}
	workflow,_:=in["workflow"].(map[string]any);if len(workflow)==0{return nil,"",0,"",errors.New("workflow is required")}
	runtime,err:=h.comfyRuntime();if err!=nil{return nil,"",0,"",err}
	validation,err:=runtime.ValidateWorkflow(ctx,workflow);if err!=nil{return nil,"",0,"",err}
	if !validation.Compatible{return nil,"",0,"",errors.New(strings.Join(validation.Errors,"; "))}
	prompt,_:=in["prompt"].(string);prompt=strings.TrimSpace(prompt);if prompt==""{return nil,"",0,"",errors.New("prompt is required")}
	params:=assistantCloneMap(in["parameters"])
	if params==nil{params=map[string]any{}}
	params["validate_runtime"]=true
	if inputAssets,ok:=in["input_assets"].(map[string]any);ok&&len(inputAssets)>0{
		storageKeys:=map[string]any{}
		for name,value:=range inputAssets{
			assetID,ok:=value.(string);if !ok||strings.TrimSpace(assetID)==""{return nil,"",0,"",fmt.Errorf("input asset %s must be an asset UUID",name)}
			id,parseErr:=uuid.Parse(assetID);if parseErr!=nil{return nil,"",0,"",fmt.Errorf("input asset %s has invalid UUID",name)}
			if h.assets==nil{return nil,"",0,"",errors.New("asset store is not configured")}
			asset,assetErr:=h.assets.GetOwned(ctx,userID,projectID,id);if assetErr!=nil{return nil,"",0,"",fmt.Errorf("input asset %s is not owned by the project: %w",name,assetErr)}
			storageKeys[name]=asset.StorageKey
		}
		params["asset_storage_keys"]=storageKeys
	}
	references:=[]uuid.UUID{}
	if raw,ok:=in["reference_ids"].([]any);ok{for _,value:=range raw{if text,ok:=value.(string);ok{if id,parseErr:=uuid.Parse(text);parseErr==nil{references=append(references,id)}}}}
	req:=generation.Request{ProjectID:projectID,Prompt:prompt,NegativePrompt:assistantStringValue(in["negative_prompt"]),AspectRatio:assistantStringValue(in["aspect_ratio"]),Parameters:params,ReferenceIDs:references,IdempotencyKey:key,ResolvedWorkflow:workflow,ResolvedWorkflowHash:hashWorkflow(workflow),FinalParameters:params}
	if seed,ok:=in["seed"].(float64);ok{n:=int64(seed);req.Seed=&n}
	store,err:=generation.NewSQLStore(h.db);if err!=nil{return nil,"",0,"",err}
	item,created,err:=store.Create(ctx,userID,req,h.provider.Name(),h.provider.Model());if err!=nil{return nil,"",0,"",err}
	if created{task,taskErr:=generation.NewTask(userID,item.ID);if taskErr!=nil{return nil,"",0,"",taskErr};if _,queueErr:=h.queue.Enqueue(ctx,task,asynq.MaxRetry(5),asynq.Timeout(6*time.Minute),asynq.Retention(24*time.Hour));queueErr!=nil{_ = store.MarkFailed(ctx,userID,item.ID,"queue_failed",queueErr.Error(),time.Now());return nil,"",0,"",queueErr}}
	return map[string]any{"generation":item,"created":created,"compatibility":map[string]any{"compatible":true,"warnings":validation.Warnings,"referenced_nodes":validation.ReferencedNodes,"referenced_models":validation.ReferencedModels}},"The flow was runtime-validated and queued through the existing generation worker.",0.99,"The generation record stores the resolved workflow and hash.",nil
}

func (h *Handler) findReusableRecipe(ctx context.Context,userID,projectID uuid.UUID,task string,runtime comfyFlowRuntime)(map[string]any,bool,error){
	query := "SELECT r.id,r.name,r.description,r.model,r.model_version,r.current_version,rv.workflow,rv.default_parameters FROM recipes r JOIN recipe_versions rv ON rv.recipe_id=r.id AND rv.version=r.current_version WHERE r.provider='comfyui' AND ((r.user_id=$1 AND r.project_id=$2) OR (r.user_id=$1 AND r.scope='global') OR (r.scope='global' AND r.published=TRUE)) ORDER BY r.updated_at DESC LIMIT 50"
	rows,err:=h.db.QueryContext(ctx,query,userID,projectID);if err!=nil{return nil,false,err};defer func(){_=rows.Close()}()
	keywords:=taskKeywords(task)
	for rows.Next(){
		var id uuid.UUID;var name,description,model,modelVersion string;var version int;var wfRaw,paramRaw []byte
		if err:=rows.Scan(&id,&name,&description,&model,&modelVersion,&version,&wfRaw,&paramRaw);err!=nil{return nil,false,err}
		score:=stringScore(name+" "+description,keywords);if score<1{continue}
		var wf map[string]any;if json.Unmarshal(wfRaw,&wf)!=nil||len(wf)==0{continue}
		validation,err:=runtime.ValidateWorkflow(ctx,wf);if err!=nil{return nil,false,err};if !validation.Compatible{continue}
		var params map[string]any;_=json.Unmarshal(paramRaw,&params);if params==nil{params=map[string]any{}}
		return map[string]any{"name":name,"description":description,"prompt":task,"negative_prompt":"","inputs":[]any{},"parameters":params,"selected_model":map[string]string{"filename":model,"version":modelVersion},"workflow":wf,"reasoning":"Reused an existing recipe after validating it against the current ComfyUI runtime.","source":"existing_recipe","recipe_id":id,"recipe_version":version,"compatibility":map[string]any{"compatible":true,"errors":validation.Errors,"warnings":validation.Warnings,"referenced_nodes":validation.ReferencedNodes,"referenced_models":validation.ReferencedModels}},true,nil
	}
	return nil,false,rows.Err()
}

func hashWorkflow(workflow map[string]any) string {data,_:=json.Marshal(workflow);sum:=sha256.Sum256(data);return fmt.Sprintf("%x",sum[:])}

func assistantCloneMap(input any) map[string]any {
	source,ok:=input.(map[string]any);if !ok{return nil};out:=map[string]any{};for key,value:=range source{out[key]=value};return out
}
func assistantStringValue(input any) string {value,_:=input.(string);return strings.TrimSpace(value)}

func (h *Handler) decide(w http.ResponseWriter,r *http.Request){
	p,ok:=auth.PrincipalFromContext(r.Context());if !ok{errJSON(w,401,"unauthorized","authentication required");return}
	pid,e:=uuid.Parse(r.PathValue("project_id"));if e!=nil{errJSON(w,400,"invalid_project_id","invalid project id");return}
	aid,e:=uuid.Parse(r.PathValue("action_id"));if e!=nil{errJSON(w,400,"invalid_action_id","invalid action id");return}
	decisionKey:=strings.TrimSpace(r.Header.Get("Idempotency-Key"));if decisionKey==""{errJSON(w,400,"missing_idempotency_key","Idempotency-Key header is required");return}
	if existing,e:=h.actions.GetByDecisionIdempotencyKey(r.Context(),p.UserID,pid,decisionKey);e==nil{writeJSON(w,200,existing);return}
	var in struct{Decision string `json:"decision"`;FinalText *string `json:"final_text,omitempty"`;Components map[string]string `json:"components,omitempty"`}
	if e=decode(r,&in);e!=nil{errJSON(w,400,"invalid_request","invalid recommendation decision");return}
	current,e:=h.actions.GetOwned(r.Context(),p.UserID,pid,aid);if errors.Is(e,sql.ErrNoRows){errJSON(w,404,"assistant_action_not_found","assistant action not found");return};if e!=nil{errJSON(w,500,"assistant_action_failed","could not load assistant action");return}
	if current.Kind!=kindRecommendation||current.Decision!=nil{errJSON(w,409,"recommendation_already_decided","recommendation is already decided");return}
	decision:=strings.ToLower(strings.TrimSpace(in.Decision));output:=clone(current.Output)
	switch decision{
	case "apply":
		switch current.Tool{
		case "suggest_prompt":e=h.applyPrompt(r.Context(),p.UserID,pid,output)
		case "analyze_composition":e=h.applyComposition(r.Context(),p.UserID,pid,output)
		case "suggest_materials":
			output["decision_effect"] = "accepted_without_automatic_mutation"
			output["project_mutated"] = false
		default:e=errors.New("this recommendation has no automatic apply operation; use ignore")
		}
		if e!=nil{errJSON(w,409,"recommendation_apply_failed",e.Error());return}
	case "edit":
		if current.Tool!="suggest_prompt"||h.prompts==nil||strings.TrimSpace(valueOr(in.FinalText))==""{errJSON(w,400,"edit_payload_required","prompt recommendations require final_text for edit");return}
		var iter,parent *uuid.UUID
		if v,ok:=output["iteration_id"].(string);ok&&v!=""{if x,pe:=uuid.Parse(v);pe==nil{iter=&x}}
		if v,ok:=output["parent_prompt_id"].(string);ok&&v!=""{if x,pe:=uuid.Parse(v);pe==nil{parent=&x}}
		text:=strings.TrimSpace(*in.FinalText)
		created,pe:=h.prompts.Create(r.Context(),p.UserID,pid,prompts.Request{IterationID:iter,ParentPromptID:parent,OriginalText:text,FinalText:&text,Components:in.Components,CreatedBy:prompts.CreatedByMixed});if pe!=nil{errJSON(w,400,"prompt_edit_failed","could not persist edited recommendation");return}
		output["edited_prompt_id"]=created.ID;output["edited_prompt_version"]=created.Version
	case "ignore":
	default:errJSON(w,400,"invalid_decision","decision must be apply, edit or ignore");return
	}
	updated,e:=h.actions.Decide(r.Context(),p.UserID,pid,aid,decision,decisionKey,output);if e!=nil{errJSON(w,409,"recommendation_decision_failed","could not record recommendation decision");return}
	if h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:p.UserID,ProjectID:pid,EntityType:"assistant_action",EntityID:aid,Action:"assistant.recommendation."+decision,Payload:map[string]any{"tool":current.Tool}})}
	if h.events!=nil{_,_=h.events.Append(r.Context(),p.UserID,pid,"assistant.recommendation."+decision,"assistant_action",aid,map[string]any{"tool":current.Tool})}
	writeJSON(w,200,updated)
}

func (h *Handler) createIteration(ctx context.Context,userID,projectID uuid.UUID,in map[string]any)(map[string]any,string,float64,string,error){
	t,ok:=in["type"].(string);if !ok||strings.TrimSpace(t)==""{return nil,"",0,"",errors.New("type is required")}
	var parent *uuid.UUID;if s,_:=in["parent_iteration_id"].(string);s!=""{x,e:=uuid.Parse(s);if e!=nil{return nil,"",0,"",e};parent=&x}
	item,e:=h.iterations.Create(ctx,userID,projectID,parent,iterations.Type(t),stringPtr(in["title"]),stringPtr(in["description"]));if e!=nil{return nil,"",0,"",e}
	payload:=map[string]any{"iteration_id":item.ID,"type":item.Type,"parent_iteration_id":item.ParentIterationID}
	if h.events!=nil{_,_=h.events.Append(ctx,userID,projectID,"iteration.created","iteration",item.ID,payload)}
	if h.provenance!=nil{_,_=h.provenance.Append(ctx,provenance.Event{UserID:userID,ProjectID:projectID,IterationID:&item.ID,EntityType:"iteration",EntityID:item.ID,Action:"assistant.iteration.created",Payload:payload})}
	return map[string]any{"iteration":item},"Creates a new immutable history node.",0.98,"The tool does not infer missing creative context.",nil
}

func (h *Handler) history(ctx context.Context,userID,projectID uuid.UUID)(map[string]any,string,float64,string,error){
	var id uuid.UUID;var name,status string
	if e:=h.db.QueryRowContext(ctx,`SELECT id,name,status FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted'`,projectID,userID).Scan(&id,&name,&status);e!=nil{return nil,"",0,"",e}
	it,e:=h.iterations.List(ctx,userID,projectID);if e!=nil{return nil,"",0,"",e}
	out:=map[string]any{"project":map[string]any{"id":id,"name":name,"status":status},"iterations":it}
	if h.prompts!=nil{if ps,pe:=h.prompts.List(ctx,userID,projectID);pe==nil{out["prompts"]=ps}}
	var gens []map[string]any
	rows,e:=h.db.QueryContext(ctx,`SELECT id,iteration_id,prompt,status,provider,model,created_at FROM generations WHERE project_id=$1 AND user_id=$2 ORDER BY created_at ASC,id ASC`,projectID,userID)
	if e==nil{defer func() { _ = rows.Close() }();for rows.Next(){var gid uuid.UUID;var iid *uuid.UUID;var prompt,st,provider,model string;var at time.Time;if rows.Scan(&gid,&iid,&prompt,&st,&provider,&model,&at)==nil{gens=append(gens,map[string]any{"id":gid,"iteration_id":iid,"prompt":prompt,"status":st,"provider":provider,"model":model,"created_at":at})}};out["generations"]=gens}
	return out,"History is assembled from fixed ownership-checked domain queries.",0.97,"Absent optional records are not invented.",nil
}

func (h *Handler) analyzeComposition(ctx context.Context,userID,projectID uuid.UUID,in map[string]any)(map[string]any,string,float64,string,error){
	if h.assets==nil||h.storage==nil{return nil,"",0,"",errors.New("asset analysis is not configured")}
	s,_:=in["asset_id"].(string);aid,e:=uuid.Parse(s);if e!=nil{return nil,"",0,"",errors.New("asset_id is required")}
	a,e:=h.assets.GetOwned(ctx,userID,projectID,aid);if e!=nil{return nil,"",0,"",e};obj,_,e:=h.storage.Get(ctx,a.StorageKey);if e!=nil{return nil,"",0,"",e};defer func() { _ = obj.Close() }()
	data,e:=io.ReadAll(io.LimitReader(obj,assets.MaxAssetSize+1));if e!=nil{return nil,"",0,"",e};if int64(len(data))>assets.MaxAssetSize{return nil,"",0,"",errors.New("asset exceeds analysis limit")}
	out,e:=similarity.AnalyzeComposition(data);if e!=nil{return nil,"",0,"",e};out["asset_id"]=aid
	if s,_:=in["iteration_id"].(string);s!=""{if iid,pe:=uuid.Parse(s);pe==nil{out["iteration_id"]=iid.String()}}else{var latest uuid.UUID;if h.db.QueryRowContext(ctx,`SELECT id FROM iterations WHERE project_id=$1 ORDER BY created_at DESC LIMIT 1`,projectID).Scan(&latest)==nil{out["iteration_id"]=latest.String()}}
	return out,"Composition analysis is deterministic and explicitly heuristic.",0.62,"No object detector, segmentation model, or learned vision model is used.",nil
}

func (h *Handler) searchReferences(ctx context.Context,userID,projectID uuid.UUID,in map[string]any)(map[string]any,string,float64,string,error){
	if h.refs==nil{return nil,"",0,"",errors.New("reference store is not configured")}
	items,e:=h.refs.List(ctx,userID,projectID);if e!=nil{return nil,"",0,"",e};q,_:=in["query"].(string);q=strings.ToLower(strings.TrimSpace(q))
	out:=make([]references.Reference,0,len(items));for _,item:=range items{hay:=strings.ToLower(strings.Join([]string{item.License,valueOr(item.Notes),string(item.SourceType),valueOr(item.SourceURL)}," "));if q==""||strings.Contains(hay,q){out=append(out,item)}}
	sort.Slice(out,func(i,j int)bool{return out[i].CreatedAt.After(out[j].CreatedAt)})
	return map[string]any{"scope":"project_references","references":out,"count":len(out)},"Only persisted project references are searched.",0.93,"No external web index or internet-wide search is queried.",nil
}

func (h *Handler) checkSimilarity(ctx context.Context,userID,projectID uuid.UUID,in map[string]any)(map[string]any,string,float64,string,error){
	if h.refs==nil||h.assets==nil||h.storage==nil{return nil,"",0,"",errors.New("similarity is not configured")}
	s,_:=in["target_asset_id"].(string);targetID,e:=uuid.Parse(s);if e!=nil{return nil,"",0,"",errors.New("target_asset_id is required")}
	target,e:=h.assets.GetOwned(ctx,userID,projectID,targetID);if e!=nil{return nil,"",0,"",e};obj,_,e:=h.storage.Get(ctx,target.StorageKey);if e!=nil{return nil,"",0,"",e};defer func() { _ = obj.Close() }();td,e:=io.ReadAll(io.LimitReader(obj,assets.MaxAssetSize+1));if e!=nil{return nil,"",0,"",e}
	refs,e:=h.refs.List(ctx,userID,projectID);if e!=nil{return nil,"",0,"",e};var maxV,maxC,maxS,maxSt float64;var sources,unavailable []string
	for _,ref:=range refs{sources=append(sources,ref.ID.String());if ref.AssetID==nil{unavailable=append(unavailable,ref.ID.String());continue};ra,e:=h.assets.GetOwned(ctx,userID,projectID,*ref.AssetID);if e!=nil{unavailable=append(unavailable,ref.ID.String());continue};ro,_,e:=h.storage.Get(ctx,ra.StorageKey);if e!=nil{unavailable=append(unavailable,ref.ID.String());continue};rd,e:=io.ReadAll(io.LimitReader(ro,assets.MaxAssetSize+1));_=ro.Close();if e!=nil{unavailable=append(unavailable,ref.ID.String());continue};res,e:=similarity.Analyze(rd,td);if e!=nil{unavailable=append(unavailable,ref.ID.String());continue};if res.Visual>maxV{maxV=res.Visual};if res.Composition>maxC{maxC=res.Composition};if res.Semantic>maxS{maxS=res.Semantic};if res.Style>maxSt{maxSt=res.Style}}
	return map[string]any{"target_asset_id":targetID,"visual_score":maxV,"composition_score":maxC,"semantic_score":maxS,"style_score":maxSt,"search_scope":"project_references","sources":sources,"unavailable_sources":unavailable,"checked_at":time.Now().UTC(),"algorithm":"deterministic-image-descriptors","algorithm_version":similarity.AlgorithmVersion},"Scores are maximums across available project references.",0.86,"Similarity does not establish authorship, infringement, uniqueness, or complete internet coverage.",nil
}

func (h *Handler) suggestPrompt(ctx context.Context,userID,projectID uuid.UUID,in map[string]any)(map[string]any,string,float64,string,error){
	if h.prompts==nil{return nil,"",0,"",errors.New("prompt store is not configured")}
	items,e:=h.prompts.List(ctx,userID,projectID);if e!=nil{return nil,"",0,"",e};base,_:=in["text"].(string);var parent,iter *uuid.UUID
	if len(items)>0{last:=items[len(items)-1];base=last.OriginalText;if last.FinalText!=nil&&strings.TrimSpace(*last.FinalText)!=""{base=*last.FinalText};parent=&last.ID;iter=last.IterationID}
	base=strings.TrimSpace(base);if base==""{base="Describe the desired image"};s:=base;lower:=strings.ToLower(s);if !strings.Contains(lower,"focal"){s+=", emphasize a clear focal point"};if !strings.Contains(lower,"lighting"){s+=", use controlled lighting"};if !strings.Contains(lower,"negative"){s+=", preserve intentional negative space"}
	return map[string]any{"suggested_prompt":s,"parent_prompt_id":uuidString(parent),"iteration_id":uuidString(iter)},"The suggestion is generated by deterministic prompt rules from stored prompt context.",0.63,"No language model or external knowledge source is used in this MVP.",nil
}

func (h *Handler) suggestMaterials(ctx context.Context,userID,projectID uuid.UUID,in map[string]any)(map[string]any,string,float64,string,error){
	kind,_:=in["kind"].(string);if kind==""{kind="material"};if kind!="material"&&kind!="texture"{return nil,"",0,"",errors.New("kind must be material or texture")}
	q,_:=in["query"].(string);q=strings.TrimSpace(q)
	query:=`SELECT id,user_id,kind,category,name,description,tags,prompt_fragment,preview_key,created_at,updated_at FROM library_items WHERE kind=$1 AND (user_id IS NULL OR user_id=$2)`;args:=[]any{kind,userID};if q!=""{query+=` AND (name ILIKE $3 OR COALESCE(description,'') ILIKE $3 OR prompt_fragment ILIKE $3)`;args=append(args,"%"+q+"%")};query+=" ORDER BY user_id NULLS FIRST,created_at DESC LIMIT 20"
	rows,e:=h.db.QueryContext(ctx,query,args...);if e!=nil{return nil,"",0,"",e};defer func() { _ = rows.Close() }();var items []library.Item
	for rows.Next(){var x library.Item;var tags []byte;if e:=rows.Scan(&x.ID,&x.UserID,&x.Kind,&x.Category,&x.Name,&x.Description,&tags,&x.PromptFragment,&x.PreviewKey,&x.CreatedAt,&x.UpdatedAt);e==nil{_ = json.Unmarshal(tags,&x.Tags);items=append(items,x)}}
	return map[string]any{"scope":"project_visible_library","kind":kind,"items":items},"Suggestions come from the project-visible library.",0.90,"Ranking is search/recency based, not a learned preference model.",nil
}

func (h *Handler) createGeneration(ctx context.Context,userID,projectID uuid.UUID,in map[string]any,key string)(map[string]any,string,float64,string,error){
	if h.queue==nil||h.provider==nil{return nil,"",0,"",errors.New("generation provider is not configured")}
	prompt,_:=in["prompt"].(string);prompt=strings.TrimSpace(prompt);if prompt==""{return nil,"",0,"",errors.New("prompt is required")}
	var iter *uuid.UUID;if v,_:=in["iteration_id"].(string);v!=""{x,e:=uuid.Parse(v);if e!=nil{return nil,"",0,"",e};iter=&x}
	req:=generation.Request{ProjectID:projectID,IterationID:iter,Prompt:prompt,IdempotencyKey:key};if v,_:=in["negative_prompt"].(string);v!=""{req.NegativePrompt=v};if v,_:=in["aspect_ratio"].(string);v!=""{req.AspectRatio=v};if v,ok:=in["seed"].(float64);ok{n:=int64(v);req.Seed=&n};if v,ok:=in["parameters"].(map[string]any);ok{req.Parameters=v}
	store,e:=generation.NewSQLStore(h.db);if e!=nil{return nil,"",0,"",e};item,created,e:=store.Create(ctx,userID,req,h.provider.Name(),h.provider.Model());if e!=nil{return nil,"",0,"",e};if created{task,e:=generation.NewTask(userID,item.ID);if e!=nil{return nil,"",0,"",e};if _,e=h.queue.Enqueue(ctx,task,asynq.MaxRetry(5),asynq.Timeout(6*time.Minute),asynq.Retention(24*time.Hour));e!=nil{return nil,"",0,"",e}}
	return map[string]any{"generation":item,"created":created},"Uses the same generation store, queue, and provider as the normal API.",0.99,"A queued generation is not presented as a completed image.",nil
}

func (h *Handler) compareIterations(ctx context.Context,userID,projectID uuid.UUID,in map[string]any)(map[string]any,string,float64,string,error){
	aID,_:=in["iteration_a_id"].(string)
	bID,_:=in["iteration_b_id"].(string)
	a,e:=uuid.Parse(aID);if e!=nil{return nil,"",0,"",errors.New("iteration_a_id is required")}
	b,e:=uuid.Parse(bID);if e!=nil{return nil,"",0,"",errors.New("iteration_b_id is required")}
	if a==b{return nil,"",0,"",errors.New("iterations must be different")}
	ia,e:=h.iterations.Get(ctx,userID,a);if e!=nil||ia.ProjectID!=projectID{return nil,"",0,"",errors.New("iteration_a_id not found")}
	ib,e:=h.iterations.Get(ctx,userID,b);if e!=nil||ib.ProjectID!=projectID{return nil,"",0,"",errors.New("iteration_b_id not found")}

	assetA,e:=h.loadCriticAsset(ctx,userID,projectID,a)
	if e!=nil{return nil,"",0,"",e}
	assetB,e:=h.loadCriticAsset(ctx,userID,projectID,b)
	if e!=nil{return nil,"",0,"",e}

	promptScore,promptAvailable:=promptSimilarity(assetA.Prompt,assetB.Prompt)
	comparedAt:=time.Now().UTC()
	out:=map[string]any{
		"a":ia,
		"b":ib,
		"compared_at":comparedAt,
		"assets":map[string]any{
			"a":map[string]any{"id":assetA.ID,"available":assetA.HasAsset},
			"b":map[string]any{"id":assetB.ID,"available":assetB.HasAsset},
		},
		"prompts":map[string]any{
			"a":assetA.Prompt,
			"b":assetB.Prompt,
			"similarity":nil,
		},
	}
	if promptAvailable {
		out["prompts"].(map[string]any)["similarity"]=promptScore
	}
	observations:=[]map[string]any{}
	if valueOrString(ia.Title)!=valueOrString(ib.Title)||valueOrString(ia.Description)!=valueOrString(ib.Description)||ia.Type!=ib.Type{
		observations=append(observations,criticObservation("iteration","Iteration metadata differs.","Stored iteration title, description, and type are compared directly.",0.99))
	}

	if !assetA.HasAsset||!assetB.HasAsset||h.storage==nil{
		observations=append(observations,buildCriticPromptObservation(assetA.Prompt,assetB.Prompt))
		observations=append(observations,criticObservation("visual","Visual and composition comparison is unavailable.","Both iterations must have an active generated image and configured object storage.",0.99))
		out["observations"]=observations
		return out,"Comparison uses persisted iteration metadata and prompt text; visual/composition descriptors are included when both active images are available.",0.66,"Visual/composition observations are unavailable when an active generated image is missing; no visual similarity is inferred. Missing prompt text is reported as unavailable.",nil
	}

	dataA,e:=h.readCriticObject(ctx,assetA.StorageKey);if e!=nil{return nil,"",0,"",e}
	dataB,e:=h.readCriticObject(ctx,assetB.StorageKey);if e!=nil{return nil,"",0,"",e}
	result,e:=similarity.Analyze(dataA,dataB);if e!=nil{return nil,"",0,"",e}
	compA,e:=similarity.AnalyzeComposition(dataA);if e!=nil{return nil,"",0,"",e}
	compB,e:=similarity.AnalyzeComposition(dataB);if e!=nil{return nil,"",0,"",e}

	out["image_comparison"]=map[string]any{
		"visual_similarity":result.Visual,
		"composition_similarity":result.Composition,
		"semantic_similarity_proxy":result.Semantic,
		"style_similarity":result.Style,
		"perceptual_hash_score":result.PHashScore,
		"histogram_score":result.HistogramScore,
		"embedding_score":result.EmbeddingScore,
		"algorithm":"deterministic-image-descriptors",
		"algorithm_version":similarity.AlgorithmVersion,
	}
	out["visual_features"]=map[string]any{
		"a":criticVisualFeatures(result.Reference),
		"b":criticVisualFeatures(result.Target),
	}
	out["composition"]=map[string]any{"a":compA,"b":compB}
	out["observations"]=append(observations,buildCriticObservations(result,compA,compB,assetA.Prompt,assetB.Prompt)...)
	return out,"Critic comparison combines persisted iteration metadata, prompt text, and deterministic visual/composition descriptors.",0.84,"This is a heuristic critic: semantic similarity is only a proxy from deterministic image descriptors; it does not use object detection or claim legal similarity/uniqueness.",nil
}

func (h *Handler) loadCriticAsset(ctx context.Context,userID,projectID,iterationID uuid.UUID)(criticAsset,error){
	var result criticAsset
	var prompt,original sql.NullString
	promptErr:=h.db.QueryRowContext(ctx,"SELECT final_text,original_text FROM prompts WHERE iteration_id=$1 AND project_id=$2 ORDER BY version DESC LIMIT 1",iterationID,projectID).Scan(&prompt,&original)
	if promptErr!=nil && !errors.Is(promptErr,sql.ErrNoRows){return criticAsset{},promptErr}
	if prompt.Valid{result.Prompt=strings.TrimSpace(prompt.String)}else if original.Valid{result.Prompt=strings.TrimSpace(original.String)}
	var generationPrompt,assetID,storageKey sql.NullString
	err:=h.db.QueryRowContext(ctx,
		"SELECT g.prompt,a.id,a.storage_key "+
			"FROM generations g "+
			"LEFT JOIN LATERAL ("+
				"SELECT id,storage_key FROM assets "+
				"WHERE generation_id=g.id AND lifecycle_status='active' "+
				"ORDER BY created_at DESC,id DESC LIMIT 1"+
			") a ON TRUE "+
			"WHERE g.iteration_id=$1 AND g.project_id=$2 AND g.user_id=$3 "+
			"ORDER BY g.created_at DESC,g.id DESC LIMIT 1",
		iterationID,projectID,userID).Scan(&generationPrompt,&assetID,&storageKey)
	if errors.Is(err,sql.ErrNoRows){return result,nil}
	if err!=nil{return criticAsset{},err}
	if result.Prompt==""&&generationPrompt.Valid{result.Prompt=strings.TrimSpace(generationPrompt.String)}
	if assetID.Valid&&storageKey.Valid{
		result.ID=assetID.String
		result.StorageKey=storageKey.String
		result.HasAsset=true
	}
	return result,nil
}

func (h *Handler) readCriticObject(ctx context.Context,key string)([]byte,error){
	obj,_,err:=h.storage.Get(ctx,key)
	if err!=nil{return nil,err}
	defer func(){_=obj.Close()}()
	data,err:=io.ReadAll(io.LimitReader(obj,assets.MaxAssetSize+1))
	if err!=nil{return nil,err}
	if int64(len(data))>assets.MaxAssetSize{return nil,errors.New("asset exceeds analysis limit")}
	return data,nil
}

func (h *Handler) verifyProvenance(ctx context.Context,userID,projectID uuid.UUID)(map[string]any,string,float64,string,error){
	if h.provenance==nil{return nil,"",0,"",errors.New("provenance is not configured")};v,e:=h.provenance.Verify(ctx,userID,projectID);if e!=nil{return nil,"",0,"",e}
	return map[string]any{"valid":v.Valid,"events_checked":v.EventsChecked,"broken_links":v.BrokenLinks,"verified_at":v.VerifiedAt},"The result is read from persisted hash-chain verification.",1.0,"Integrity verification does not prove authorship or external originality.",nil
}

func (h *Handler) applyPrompt(ctx context.Context,userID,projectID uuid.UUID,out map[string]any)error{
	if h.prompts==nil{return errors.New("prompt store is not configured")};text,_:=out["suggested_prompt"].(string);text=strings.TrimSpace(text);if text==""{return errors.New("suggested prompt is empty")}
	var parent,iter *uuid.UUID;if v,_:=out["parent_prompt_id"].(string);v!=""{if x,e:=uuid.Parse(v);e==nil{parent=&x}};if v,_:=out["iteration_id"].(string);v!=""{if x,e:=uuid.Parse(v);e==nil{iter=&x}}
	p,e:=h.prompts.Create(ctx,userID,projectID,prompts.Request{IterationID:iter,ParentPromptID:parent,OriginalText:text,FinalText:&text,AISuggestions:[]string{text},CreatedBy:prompts.CreatedByAI});if e!=nil{return e};out["applied_prompt_id"]=p.ID;out["applied_prompt_version"]=p.Version
	if h.provenance!=nil{_,_=h.provenance.Append(ctx,provenance.Event{UserID:userID,ProjectID:projectID,IterationID:iter,EntityType:"prompt",EntityID:p.ID,Action:"assistant.prompt.applied",Payload:map[string]any{"version":p.Version}})}
	return nil
}

func (h *Handler) applyComposition(ctx context.Context,userID,projectID uuid.UUID,out map[string]any)error{
	s,_:=out["iteration_id"].(string);iid,e:=uuid.Parse(s);if e!=nil{return errors.New("iteration_id is required")}
	var owned bool;if e=h.db.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM iterations i JOIN projects p ON p.id=i.project_id WHERE i.id=$1 AND i.project_id=$2 AND p.user_id=$3 AND p.status <> 'deleted')`,iid,projectID,userID).Scan(&owned);e!=nil||!owned{return errors.New("iteration not found")}
	raw:=func(k string)[]byte{if v:=out[k];v!=nil{b,_:=json.Marshal(v);if len(b)>0&&string(b)!="null"{return b}};return []byte("{}")}
	_,e=h.db.ExecContext(ctx,`INSERT INTO composition_specs(project_id,iteration_id,user_id,focal_points,bounding_boxes,relative_positions,horizon,camera_elevation,perspective,hierarchy,negative_space,dominant_geometry,object_scale,light_direction) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) ON CONFLICT(iteration_id) DO UPDATE SET focal_points=excluded.focal_points,bounding_boxes=excluded.bounding_boxes,relative_positions=excluded.relative_positions,horizon=excluded.horizon,camera_elevation=excluded.camera_elevation,perspective=excluded.perspective,hierarchy=excluded.hierarchy,negative_space=excluded.negative_space,dominant_geometry=excluded.dominant_geometry,object_scale=excluded.object_scale,light_direction=excluded.light_direction,updated_at=now()`,projectID,iid,userID,raw("focal_points"),raw("bounding_boxes"),raw("relative_positions"),out["horizon"],out["camera_elevation"],stringValue(out["perspective"]),raw("hierarchy"),raw("negative_space"),raw("dominant_geometry"),raw("object_scale"),raw("light_direction"))
	if e!=nil{return e};out["applied_iteration_id"]=iid
	if h.provenance!=nil{_,_=h.provenance.Append(ctx,provenance.Event{UserID:userID,ProjectID:projectID,IterationID:&iid,EntityType:"composition",EntityID:iid,Action:"assistant.composition.applied",Payload:map[string]any{"source":"assistant"}})}
	return nil
}

func withRecommendation(tool string, result map[string]any, reason string, confidence float64, uncertainty string) map[string]any {
	evidence:=clone(result)
	affected:=map[string]any{"type":"project"}
	if id,ok:=result["asset_id"];ok{affected=map[string]any{"type":"asset","id":id}}
	if id,ok:=result["iteration_id"];ok{affected=map[string]any{"type":"iteration","id":id}}
	recommendation:=map[string]any{
		"recommendation": recommendationText(tool, result),
		"reason": reason,
		"evidence": evidence,
		"confidence": confidence,
		"affected_entity": affected,
		"expected_effect": expectedEffect(tool),
	}
	result["recommendation"]=recommendation
	return result
}

func recommendationText(tool string, result map[string]any) string {
	switch tool {
	case "suggest_prompt":
		if v,ok:=result["suggested_prompt"].(string);ok{return v}
	case "suggest_materials":
		return "Review the suggested project-visible library items for the requested material or texture."
	case "analyze_composition":
		return "Review the detected composition descriptors and optionally apply them to the selected iteration."
	}
	return "Review this AI recommendation before applying it."
}

func expectedEffect(tool string) string {
	switch tool {
	case "suggest_prompt": return "Create a new prompt version based on the proposed refinement."
	case "suggest_materials": return "Accept the suggested project-visible materials or textures for later manual selection; no project mutation occurs automatically."
	case "analyze_composition": return "Persist the reviewed composition representation without overwriting immutable history."
	}
	return "No project mutation occurs until the user explicitly decides to apply or edit it."
}

func (h *Handler) ownedProject(ctx context.Context,userID,projectID uuid.UUID)bool{
	var exists bool
	err:=h.db.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')`,projectID,userID).Scan(&exists)
	return err==nil&&exists
}

func descriptor(name string)*ToolDescriptor{for i:=range toolCatalog{if toolCatalog[i].Name==name{return &toolCatalog[i]}};return nil}
func decode(r *http.Request,v any)error{d:=json.NewDecoder(io.LimitReader(r.Body,1<<20));d.DisallowUnknownFields();if e:=d.Decode(v);e!=nil{return e};var extra any;if e:=d.Decode(&extra);e!=io.EOF{return errors.New("multiple JSON values")};return nil}
func clone(in map[string]any)map[string]any{out:=map[string]any{};for k,v:=range in{out[k]=v};return out}
func stringPtr(v any)*string{if s,ok:=v.(string);ok&&strings.TrimSpace(s)!=""{s=strings.TrimSpace(s);return &s};return nil}
func valueOr(v *string)string{if v==nil{return ""};return *v}
func valueOrString(v *string)string{if v==nil{return ""};return *v}
func uuidString(v *uuid.UUID)any{if v==nil{return nil};return v.String()}
func stringValue(v any)string{if s,ok:=v.(string);ok{return strings.TrimSpace(s)};return ""}
func parsePathUUID(r *http.Request,name string)uuid.UUID{id,e:=uuid.Parse(r.PathValue(name));if e!=nil{return uuid.Nil};return id}
func errJSON(w http.ResponseWriter,status int,code,msg string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":msg,"request_id":uuid.NewString()}})}
func writeJSON(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_=json.NewEncoder(w).Encode(v)}


func buildCriticPromptObservation(promptA, promptB string) map[string]any {
	if score, ok := promptSimilarity(promptA, promptB); ok {
		return criticObservation("prompt", formatPromptObservation(score), "Prompt similarity is a token-set Jaccard comparison of persisted prompt text.", 0.95)
	}
	return criticObservation("prompt", "Prompt comparison is unavailable.", "At least one iteration has no persisted prompt text, so no similarity score is inferred.", 0.99)
}

func criticVisualFeatures(stats similarity.Stats) map[string]any {
	return map[string]any{
		"width": stats.Width,
		"height": stats.Height,
		"aspect_ratio": stats.Aspect,
		"luminance_variance": stats.Variance,
		"edge_density": stats.EdgeDensity,
		"luminance_center": map[string]any{
			"x": stats.CenterX,
			"y": stats.CenterY,
		},
	}
}
