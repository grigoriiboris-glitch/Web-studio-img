package comfyui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type client struct {
	base string
	http *http.Client
}

func newClient(base string, timeout time.Duration) (*client, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" { return nil, fmt.Errorf("comfyui endpoint is required") }
	if _, err := url.ParseRequestURI(base); err != nil { return nil, fmt.Errorf("invalid comfyui endpoint: %w", err) }
	if timeout <= 0 { timeout = 30 * time.Second }
	return &client{base: base, http: &http.Client{Timeout: timeout}}, nil
}

func (c *client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body); if err != nil { return err }
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, r)
	if err != nil { return err }
	if body != nil { req.Header.Set("Content-Type", "application/json") }
	resp, err := c.http.Do(req)
	if err != nil { return fmt.Errorf("%w: %v", ErrProviderUnavailable, err) }
	defer func(){_ = resp.Body.Close()}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
			return fmt.Errorf("%w: status=%d body=%s", ErrProviderInvalid, resp.StatusCode, strings.TrimSpace(string(body)))
		}
		return fmt.Errorf("%w: status=%d body=%s", ErrProviderUnavailable, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil { return nil }
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil { return fmt.Errorf("decode comfyui response: %w", err) }
	return nil
}

func (c *client) upload(ctx context.Context, name, contentType string, data []byte) (uploadResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", name); if err != nil { return uploadResponse{}, err }
	if _, err = part.Write(data); err != nil { return uploadResponse{}, err }
	_ = writer.WriteField("overwrite", "true")
	_ = writer.WriteField("type", "input")
	if err := writer.Close(); err != nil { return uploadResponse{}, err }
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/upload/image", &body); if err != nil { return uploadResponse{}, err }
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if contentType != "" { req.Header.Set("X-Content-Type", contentType) }
	resp, err := c.http.Do(req); if err != nil { return uploadResponse{}, fmt.Errorf("%w: %v", ErrProviderUnavailable, err) }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { b,_:=io.ReadAll(io.LimitReader(resp.Body,16<<10)); return uploadResponse{}, fmt.Errorf("%w: upload status=%d body=%s",ErrProviderUnavailable,resp.StatusCode,strings.TrimSpace(string(b))) }
	var out uploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil { return uploadResponse{}, err }
	return out,nil
}

func (c *client) submit(ctx context.Context, workflow map[string]any, clientID string) (promptResponse,error) {
	var out promptResponse
	err:=c.doJSON(ctx,http.MethodPost,"/prompt",workflowRequest{Prompt:workflow,ClientID:clientID},&out)
	if err!=nil{return out,err}
	if out.PromptID=="" {return out,fmt.Errorf("%w: missing prompt_id",ErrProviderUnavailable)}
	if len(out.NodeErrors)>0 {return out,fmt.Errorf("%w: workflow node validation failed: %v",ErrProviderInvalid,out.NodeErrors)}
	return out,nil
}

func (c *client) history(ctx context.Context, promptID string) (historyEntry,error) {
	var raw map[string]historyEntry
	if err:=c.doJSON(ctx,http.MethodGet,"/history/"+url.PathEscape(promptID),nil,&raw);err!=nil{return historyEntry{},err}
	if item,ok:=raw[promptID];ok{return item,nil}
	var item historyEntry
	if err:=c.doJSON(ctx,http.MethodGet,"/history/"+url.PathEscape(promptID),nil,&item);err==nil{return item,nil}
	return historyEntry{},fmt.Errorf("%w: history not found for %s",ErrProviderUnavailable,promptID)
}

func (c *client) view(ctx context.Context, image outputImage) ([]byte,string,error) {
	q:=url.Values{}
	q.Set("filename",image.Filename); q.Set("subfolder",image.Subfolder); q.Set("type",image.Type)
	req,err:=http.NewRequestWithContext(ctx,http.MethodGet,c.base+"/view?"+q.Encode(),nil);if err!=nil{return nil,"",err}
	resp,err:=c.http.Do(req);if err!=nil{return nil,"",fmt.Errorf("%w: %v",ErrProviderUnavailable,err)}
	defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return nil,"",fmt.Errorf("%w: view status=%d",ErrProviderUnavailable,resp.StatusCode)}
	data,err:=io.ReadAll(io.LimitReader(resp.Body,20<<20));if err!=nil{return nil,"",err}
	return data,resp.Header.Get("Content-Type"),nil
}

func (c *client) interrupt(ctx context.Context, promptID string) error {
	body := map[string]any{}
	if strings.TrimSpace(promptID) != "" { body["prompt_id"] = promptID }
	return c.doJSON(ctx, http.MethodPost, "/interrupt", body, nil)
}
