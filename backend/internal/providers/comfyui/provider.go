package comfyui

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/oleg3190/Web-studio-img/backend/internal/generation"
	"github.com/oleg3190/Web-studio-img/backend/internal/storage"
)

type Config struct {
	Endpoint string
	Model string
	ModelVersion string
	WorkflowJSON string
	WorkflowPath string
	Timeout time.Duration
	PollEvery time.Duration
	Storage storage.StorageProvider
}

type Provider struct { cfg Config; client *client; workflow map[string]any }

func New(cfg Config) (*Provider,error) {
	if cfg.Endpoint=="" { cfg.Endpoint="http://host.docker.internal:8188" }
	if cfg.Model=="" { cfg.Model="local" }
	if cfg.Timeout<=0 { cfg.Timeout=10*time.Minute }
	if cfg.PollEvery<=0 { cfg.PollEvery=2*time.Second }
	if cfg.Storage==nil { return nil,errors.New("comfyui requires object storage") }
	workflowJSON:=strings.TrimSpace(cfg.WorkflowJSON)
	if workflowJSON=="" && cfg.WorkflowPath!="" {
		data,err:=os.ReadFile(cfg.WorkflowPath);if err!=nil{return nil,fmt.Errorf("read comfyui workflow: %w",err)}
		workflowJSON=string(data)
	}
	if workflowJSON=="" { return nil,errors.New("comfyui workflow is required (COMFYUI_WORKFLOW_JSON or COMFYUI_WORKFLOW_PATH)") }
	var workflow map[string]any
	if err:=json.Unmarshal([]byte(workflowJSON),&workflow);err!=nil{return nil,fmt.Errorf("invalid comfyui workflow JSON: %w",err)}
	c,err:=newClient(cfg.Endpoint,30*time.Second);if err!=nil{return nil,err}
	return &Provider{cfg:cfg,client:c,workflow:workflow},nil
}

func (p *Provider) Name() string { return "comfyui" }
func (p *Provider) Model() string { return p.cfg.Model }

func (p *Provider) Generate(ctx context.Context, req generation.Request) (generation.ProviderResult,error) {
	params:=req.Parameters
	operation:=stringValue(params,"operation","generate")
	inputImage,maskImage:="", ""
	width,height:=intValue(params,"width",0),intValue(params,"height",0)
	if sourceKey:=stringValue(params,"source_storage_key",""); sourceKey!="" {
		key,err:=p.assetKey(ctx,sourceKey);if err!=nil{return generation.ProviderResult{},err}
		data,info,err:=p.cfg.Storage.Get(ctx,key);if err!=nil{return generation.ProviderResult{},fmt.Errorf("load source asset: %w",err)}
		defer data.Close()
		bytes,err:=readLimited(data,20<<20);if err!=nil{return generation.ProviderResult{},err}
		upload,err:=p.client.upload(ctx,"source-"+safeName(sourceKey)+"."+extension(info.ContentType),info.ContentType,bytes);if err!=nil{return generation.ProviderResult{},err}
		inputImage=upload.Name
	}
	if maskKey:=stringValue(params,"mask_storage_key",""); maskKey!="" {
		key,err:=p.assetKey(ctx,maskKey);if err!=nil{return generation.ProviderResult{},err}
		data,info,err:=p.cfg.Storage.Get(ctx,key);if err!=nil{return generation.ProviderResult{},fmt.Errorf("load mask asset: %w",err)}
		defer data.Close()
		bytes,err:=readLimited(data,20<<20);if err!=nil{return generation.ProviderResult{},err}
		upload,err:=p.client.upload(ctx,"mask-"+safeName(maskKey)+".png",info.ContentType,bytes);if err!=nil{return generation.ProviderResult{},err}
		maskImage=upload.Name
	}
	workflow,err:=mapWorkflow(p.workflow,mapperValues{Prompt:req.Prompt,NegativePrompt:req.NegativePrompt,Seed:req.Seed,AspectRatio:req.AspectRatio,InputImage:inputImage,MaskImage:maskImage,Width:width,Height:height,Operation:operation})
	if err!=nil{return generation.ProviderResult{},err}
	clientID:=req.IdempotencyKey
	response,err:=p.client.submit(ctx,workflow,clientID);if err!=nil{return generation.ProviderResult{},err}
	pollCtx,cancel:=context.WithTimeout(ctx,p.cfg.Timeout);defer cancel()
	for {
		if err:=pollCtx.Err();err!=nil{return generation.ProviderResult{},err}
		history,err:=p.client.history(pollCtx,response.PromptID)
		if err==nil {
			if history.Status!=nil && history.Status.StatusStr=="error" {return generation.ProviderResult{},fmt.Errorf("%w: workflow failed",ErrProviderInvalid)}
			for _,node:=range history.Outputs {
				if len(node.Images)==0 {continue}
				img:=node.Images[0]
				data,mime,err:=p.client.view(pollCtx,img);if err!=nil{return generation.ProviderResult{},err}
				return generation.ProviderResult{Images:[]generation.Image{{Data:data,ContentType:normalizeMIME(mime),Filename:img.Filename}},ModelVersion:p.cfg.ModelVersion,ProviderJobID:response.PromptID},nil
			}
		}
		t:=time.NewTimer(p.cfg.PollEvery);select{case <-pollCtx.Done():t.Stop();return generation.ProviderResult{},pollCtx.Err();case <-t.C:}
	}
}

func (p *Provider) Cancel(ctx context.Context, promptID string) error {
	if err:=p.client.interrupt(ctx, promptID);err!=nil && !errors.Is(err,context.Canceled){return err}
	return nil
}

func (p *Provider) assetKey(_ context.Context,key string)(string,error) {
	key = strings.TrimSpace(key)
	if key=="" { return "", errors.New("empty asset storage key") }
	if strings.Contains(key, "\\") || strings.Contains(key, "..") { return "", errors.New("invalid asset storage key") }
	return key,nil
}

func stringValue(m map[string]any,key,def string) string { if v,ok:=m[key].(string);ok&&strings.TrimSpace(v)!=""{return v};return def }
func intValue(m map[string]any,key string,def int) int {switch v:=m[key].(type){case int:return v;case float64:return int(v);case string:n,_:=strconv.Atoi(v);if n>0{return n}};return def}
func readLimited(r io.Reader,limit int)([]byte,error){var out []byte;buf:=make([]byte,64*1024);for{n,err:=r.Read(buf);if n>0{if len(out)+n>limit{return nil,fmt.Errorf("provider input exceeds %d bytes",limit)};out=append(out,buf[:n]...)};if err!=nil{if errors.Is(err,context.Canceled){return nil,err};if errors.Is(err,io.EOF){return out,nil};return out,err}}}
func extension(mime string) string {switch mime{case "image/png":return "png";case "image/jpeg":return "jpg";default:return "img"}}
func normalizeMIME(mime string) string {if strings.HasPrefix(mime,"image/"){return mime};return "image/png"}

func safeName(value string) string { value = strings.Trim(value, "/"); value = strings.ReplaceAll(value, "/", "_"); return value }
