package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

var fxPaths = map[string]bool{"/v3/ai/language-model":true,"/v4/ai/language-model":true,"/coding-agent/v1/models":true}
const vercelGateway = "https://ai-gateway.vercel.sh"

type aiConfig struct { Key, Provider, BaseURL, Model, Source string }

func (s *Server) readAIConfig(ctx context.Context) (aiConfig,error) {
	provider:="vercel";if os.Getenv("MIAO_AI_PROVIDER")=="capi"{provider="capi"}
	cfg:=aiConfig{Key:os.Getenv("AI_GATEWAY_API_KEY"),Provider:provider,BaseURL:env("MIAO_AI_BASE_URL","http://127.0.0.1:3210/api/v1"),Model:env("MIAO_AI_MODEL","gpt-5.2"),Source:"none"}
	if cfg.Key!=""{cfg.Source="environment"}
	if saved,err:=s.PB.Find(ctx,"platform_settings","name = "+pbFilterString("ai_gateway_api_key"));err==nil{
		secret:=os.Getenv("MIAO_SETTINGS_ENCRYPTION_KEY");if len(secret)<32{return cfg,fmt.Errorf("MIAO_SETTINGS_ENCRYPTION_KEY must contain at least 32 characters")}
		key:=sha256.Sum256([]byte(secret));parts:=strings.Split(stringValue(saved["value"]),".");if len(parts)!=4||parts[0]!="v1"{return cfg,fmt.Errorf("stored AI key has unsupported format")}
		iv,e:=base64.RawURLEncoding.DecodeString(parts[1]);if e!=nil{return cfg,e};tag,e:=base64.RawURLEncoding.DecodeString(parts[2]);if e!=nil{return cfg,e};ciphertext,e:=base64.RawURLEncoding.DecodeString(parts[3]);if e!=nil{return cfg,e}
		block,e:=aes.NewCipher(key[:]);if e!=nil{return cfg,e};gcm,e:=cipher.NewGCM(block);if e!=nil{return cfg,e};if len(tag)!=gcm.Overhead(){return cfg,fmt.Errorf("stored AI key is invalid")};payload:=append(ciphertext,tag...);plain,e:=gcm.Open(nil,iv,payload,nil);if e!=nil{return cfg,e};cfg.Key=string(plain);cfg.Source="admin"
	}
	return cfg,nil
}

func (s *Server) aiRateLimited(ctx context.Context,tenantID,userID string)bool {
	start:=time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	_,tenantCount,_,e1:=s.PB.List(ctx,"ai_usage","tenant_id = "+pbFilterString(tenantID)+" && created >= "+pbFilterString(start),"",1,1)
	_,userCount,_,e2:=s.PB.List(ctx,"ai_usage","tenant_id = "+pbFilterString(tenantID)+" && user_id = "+pbFilterString(userID)+" && created >= "+pbFilterString(start),"",1,1)
	_,globalCount,_,e3:=s.PB.List(ctx,"ai_usage","created >= "+pbFilterString(start),"",1,1)
	return e1!=nil||e2!=nil||e3!=nil||tenantCount>=120||userCount>=30||globalCount>=500
}

func (s *Server) checkAIQuota(ctx context.Context,tenantID,userID string)error {
	tenant,err:=s.PB.Get(ctx,"tenants",tenantID);if err!=nil{return err}
	limit:=intValue(tenant["ai_daily_limit"])
	if limit>0 {start:=time.Now().UTC();start=time.Date(start.Year(),start.Month(),start.Day(),0,0,0,0,time.UTC);_,count,_,err:=s.PB.List(ctx,"ai_usage","tenant_id = "+pbFilterString(tenantID)+" && created >= "+pbFilterString(start.Format(time.RFC3339Nano)),"",1,1);if err!=nil{return err};if count>=limit{return fmt.Errorf("工作区已达到今日 AI 请求预算")}}
	if s.aiRateLimited(ctx,tenantID,userID){return fmt.Errorf("AI 请求次数过多，请稍后重试")};return nil
}

func (s *Server) fxGateway(w http.ResponseWriter,r *http.Request) {
	path:=r.Header.Get("X-Fx-Path");method:=r.Method
	if !fxPaths[path]||method!="GET"&&method!="POST"{writeError(w,400,"AI 请求路径无效");return}
	id:=who(r);appID:="";requested:=r.Header.Get("X-Miao-App-Id")
	if requested!=""{if app,err:=s.PB.Get(r.Context(),"apps",requested);err==nil&&app["tenant_id"]==id.Tenant["id"]&&s.appPermission(r.Context(),app,id)!=""{appID=requested}}
	config,err:=s.readAIConfig(r.Context());if err!=nil||config.Key==""{writeError(w,503,"企业尚未配置 AI 服务密钥");return}
	if err=s.checkAIQuota(r.Context(),stringValue(id.Tenant["id"]),stringValue(id.User["id"]));err!=nil{writeError(w,429,err.Error());return}
	usage,err:=s.PB.Create(r.Context(),"ai_usage",map[string]any{"tenant_id":id.Tenant["id"],"user_id":id.User["id"],"app_id":appID,"status":100,"input_tokens":0,"output_tokens":0});if err!=nil{writeError(w,503,"AI 用量记录暂不可用");return}
	var body []byte
	if method=="POST"{body,err=io.ReadAll(io.LimitReader(r.Body,8<<20));if err!=nil{_=s.PB.Update(r.Context(),"ai_usage",stringValue(usage["id"]),map[string]any{"status":400});writeError(w,400,"AI 请求内容无效");return}}
	var upstream *http.Response
	if config.Provider=="capi"{
		if method=="GET"&&path=="/coding-agent/v1/models"{
			u:=strings.TrimRight(config.BaseURL,"/")+"/models?modality=text";request,_:=http.NewRequestWithContext(r.Context(),http.MethodGet,u,nil);request.Header.Set("Authorization","Bearer "+config.Key);upstream,err=http.DefaultClient.Do(request)
		}else if method=="POST"{
			payload:=map[string]any{};if json.Unmarshal(body,&payload)!=nil{err=fmt.Errorf("AI 请求格式无效")}else{converted,e:=toOpenAIRequest(payload,config.Model);if e!=nil{err=e}else{request,_:=http.NewRequestWithContext(r.Context(),http.MethodPost,strings.TrimRight(config.BaseURL,"/")+"/chat/completions",bytes.NewReader(converted));request.Header.Set("Authorization","Bearer "+config.Key);request.Header.Set("Content-Type","application/json");request.Header.Set("Accept","text/event-stream");upstream,err=http.DefaultClient.Do(request)}}
		}else{err=fmt.Errorf("模型请求必须使用 POST")}
	}else{
		target:=vercelGateway+path
		request,reqErr:=http.NewRequestWithContext(r.Context(),method,target,bytes.NewReader(body));if reqErr!=nil{err=reqErr}else{
			for _,name:=range []string{"Accept","Content-Type","X-Vercel-AI-Data-Stream","X-Vercel-AI-SDK-Version","AI-Language-Model-Id","AI-Language-Model-Version"}{if value:=r.Header.Get(name);value!=""{request.Header.Set(name,value)}}
			request.Header.Set("Authorization","Bearer "+config.Key);upstream,err=http.DefaultClient.Do(request)
		}
	}
	if err!=nil||upstream==nil{_=s.PB.Update(r.Context(),"ai_usage",stringValue(usage["id"]),map[string]any{"status":502});writeError(w,502,"AI 服务暂时不可用");return}
	defer upstream.Body.Close()
	_=s.PB.Update(r.Context(),"ai_usage",stringValue(usage["id"]),map[string]any{"status":upstream.StatusCode})
	if config.Provider=="capi"&&method=="GET"&&upstream.StatusCode>=200&&upstream.StatusCode<300{
		data,_:=io.ReadAll(io.LimitReader(upstream.Body,4<<20));var catalog map[string]any;_=json.Unmarshal(data,&catalog);if items,ok:=catalog["data"].([]any);ok{for _,raw:=range items{if item,ok:=raw.(map[string]any);ok{item["type"]="language"}}};w.Header().Set("Content-Type","application/json");w.WriteHeader(upstream.StatusCode);_=json.NewEncoder(w).Encode(catalog);return
	}
	for _,name:=range []string{"Cache-Control","Retry-After","X-AI-Gateway-Provider","X-Vercel-AI-Gateway-Provider","X-Vercel-AI-Data-Stream"}{if v:=upstream.Header.Get(name);v!=""{w.Header().Set(name,v)}}
	w.Header().Set("Cache-Control","no-store")
	if upstream.StatusCode<200||upstream.StatusCode>=300{if value:=upstream.Header.Get("Content-Type");value!=""{w.Header().Set("Content-Type",value)};w.WriteHeader(upstream.StatusCode);_,_=io.Copy(w,io.LimitReader(upstream.Body,1<<20));return}
	if config.Provider=="capi"&&method=="POST"{w.Header().Set("Content-Type","text/event-stream; charset=utf-8");w.Header().Set("X-Vercel-AI-Data-Stream","v1");w.WriteHeader(upstream.StatusCode);usageTokens:=s.streamCAPI(w,r,upstream.Body);_,_=s.PB.Update(context.Background(),"ai_usage",stringValue(usage["id"]),map[string]any{"input_tokens":usageTokens[0],"output_tokens":usageTokens[1]});return}
	if value:=upstream.Header.Get("Content-Type");value!=""{w.Header().Set("Content-Type",value)}
	w.WriteHeader(upstream.StatusCode)
	input,output:=copyMonitored(w,r,upstream.Body);_=s.PB.Update(context.Background(),"ai_usage",stringValue(usage["id"]),map[string]any{"input_tokens":input,"output_tokens":output})
}

func toOpenAIRequest(body map[string]any,model string)([]byte,error){
	messages:=[]any{};for _,raw:=range anySlice(body["prompt"]){message:=asMap(raw);role:=stringValue(message["role"]);content:=message["content"];switch role{
	case "system","developer":messages=append(messages,map[string]any{"role":"system","content":textParts(content)})
	case "user":if text,ok:=content.(string);ok{messages=append(messages,map[string]any{"role":"user","content":text})}else{parts:=[]any{};for _,partRaw:=range anySlice(content){part:=asMap(partRaw);if part["type"]=="text"{parts=append(parts,map[string]any{"type":"text","text":part["text"]})}else if part["type"]=="image"{image:=stringValue(part["image"]);if image!=""{if !strings.HasPrefix(image,"http")&&!strings.HasPrefix(image,"data:"){image="data:"+defaultString(stringValue(part["mediaType"]),"image/png")+";base64,"+image};parts=append(parts,map[string]any{"type":"image_url","image_url":map[string]any{"url":image}})}}};messages=append(messages,map[string]any{"role":"user","content":parts})}
	case "assistant":calls:=[]any{};for _,partRaw:=range anySlice(content){part:=asMap(partRaw);if part["type"]!="tool-call"{continue};args:=part["args"];encoded,e:=json.Marshal(args);if e!=nil{return nil,e};calls=append(calls,map[string]any{"id":part["toolCallId"],"type":"function","function":map[string]any{"name":part["toolName"],"arguments":string(encoded)}})};assistant:=map[string]any{"role":"assistant","content":textParts(content)};if len(calls)>0{assistant["tool_calls"]=calls};messages=append(messages,assistant)
	case "tool":for _,partRaw:=range anySlice(content){part:=asMap(partRaw);if part["type"]=="tool-result"{value,_:=json.Marshal(part["result"]);messages=append(messages,map[string]any{"role":"tool","tool_call_id":part["toolCallId"],"name":part["toolName"],"content":string(value)})}}
	}}
	tools:=[]any{};for _,raw:=range anySlice(body["tools"]){tool:=asMap(raw);if tool["type"]=="function"&&stringValue(tool["name"])!=""{tools=append(tools,map[string]any{"type":"function","function":map[string]any{"name":tool["name"],"description":defaultString(stringValue(tool["description"]),""),"parameters":defaultMap(tool["inputSchema"],map[string]any{"type":"object","properties":map[string]any{}})}})}}
	request:=map[string]any{"model":model,"messages":messages,"stream":true,"stream_options":map[string]any{"include_usage":true}}
	if body["maxOutputTokens"]!=nil{request["max_tokens"]=body["maxOutputTokens"]};if body["temperature"]!=nil{request["temperature"]=body["temperature"]};if body["topP"]!=nil{request["top_p"]=body["topP"]};if body["stopSequences"]!=nil{request["stop"]=body["stopSequences"]}
	if len(tools)>0{request["tools"]=tools;if choice:=body["toolChoice"];choice!=nil{if text,ok:=choice.(string);ok{if containsString([]string{"auto","none","required"},text){request["tool_choice"]=text}}else{c:=asMap(choice);if c["type"]=="tool"{request["tool_choice"]=map[string]any{"type":"function","function":map[string]any{"name":c["toolName"]}}}}}}
	return json.Marshal(request)
}

func defaultMap(value any,fallback map[string]any)map[string]any{if out,ok:=value.(map[string]any);ok{return out};return fallback}
func textParts(value any)string{if text,ok:=value.(string);ok{return text};parts:=[]string{};for _,raw:=range anySlice(value){part:=asMap(raw);if part["type"]=="text"{parts=append(parts,stringValue(part["text"]))}};return strings.Join(parts,"")}

func copyMonitored(w http.ResponseWriter,r *http.Request,body io.Reader)(int,int){reader:=bufio.NewReader(body);captured:=[]byte{};buffer:=make([]byte,16*1024);flusher,_:=w.(http.Flusher);for{n,err:=reader.Read(buffer);if n>0{_,_=w.Write(buffer[:n]);if flusher!=nil{flusher.Flush()};captured=append(captured,buffer[:n]...);if len(captured)>250000{captured=append([]byte(nil),captured[len(captured)-250000:]...)}};if err!=nil{break};if r.Context().Err()!=nil{break}};return parseUsage(captured)}

func (s *Server) streamCAPI(w http.ResponseWriter,r *http.Request,body io.Reader)[2]int{
	w.Header().Set("Content-Type","text/event-stream; charset=utf-8");w.Header().Set("X-Vercel-AI-Data-Stream","v1");w.Header().Set("Cache-Control","no-store");flusher,_:=w.(http.Flusher);scanner:=bufio.NewScanner(body);scanner.Buffer(make([]byte,64*1024),1<<20);tools:=map[int]map[string]string{};textStarted:=false;finish:="stop";usage:=[2]int{};emit:=func(value any){data,_:=json.Marshal(value);_,_=fmt.Fprintf(w,"data: %s\n\n",data);if flusher!=nil{flusher.Flush()}}
	for scanner.Scan(){if r.Context().Err()!=nil{break};line:=scanner.Text();if !strings.HasPrefix(line,"data:"){continue};payload:=strings.TrimSpace(strings.TrimPrefix(line,"data:"));if payload==""||payload=="[DONE]"{continue};chunk:=map[string]any{};if json.Unmarshal([]byte(payload),&chunk)!=nil{continue};if u:=asMap(chunk["usage"]);len(u)>0{usage=[2]int{intValue(defaultAny(u["prompt_tokens"],u["input_tokens"])),intValue(defaultAny(u["completion_tokens"],u["output_tokens"]))}}
		choices:=anySlice(chunk["choices"]);if len(choices)==0{continue};choice:=asMap(choices[0]);delta:=asMap(choice["delta"]);if content:=stringValue(delta["content"]);content!=""{if !textStarted{emit(map[string]any{"type":"text-start","id":"miao-text"});textStarted=true};emit(map[string]any{"type":"text-delta","id":"miao-text","delta":content})};reasoning:=defaultString(stringValue(delta["reasoning_content"]),stringValue(delta["reasoning"]));if reasoning!=""{emit(map[string]any{"type":"reasoning","textDelta":reasoning})};for _,raw:=range anySlice(delta["tool_calls"]){item:=asMap(raw);index:=intValue(item["index"]);current:=tools[index];if current==nil{current=map[string]string{"id":"","name":"","args":""};tools[index]=current};if stringValue(item["id"])!=""{current["id"]=stringValue(item["id"])};fn:=asMap(item["function"]);if stringValue(fn["name"])!=""{current["name"]=stringValue(fn["name"])};current["args"]+=stringValue(fn["arguments"]);if current["id"]!=""&&current["name"]!=""{emit(map[string]any{"type":"tool-call-delta","toolCallType":"function","toolCallId":current["id"],"toolName":current["name"],"argsTextDelta":stringValue(fn["arguments"])})}};switch choice["finish_reason"]{case "tool_calls":finish="tool-calls";case "length":finish="length"}}
	if err:=scanner.Err();err==nil||r.Context().Err()==nil{indices:=[]int{};for i:=range tools{indices=append(indices,i)};sort.Ints(indices);for _,i:=range indices{item:=tools[i];emit(map[string]any{"type":"tool-call","toolCallType":"function","toolCallId":item["id"],"toolName":item["name"],"args":defaultString(item["args"],"{}")})};emit(map[string]any{"type":"finish","finishReason":finish,"usage":map[string]any{"promptTokens":usage[0],"completionTokens":usage[1]}})}
	return usage
}

func defaultAny(value,fallback any)any{if value!=nil{return value};return fallback}
func parseUsage(data []byte)(int,int){lines:=strings.Split(string(data),"\n");for i:=len(lines)-1;i>=0;i--{line:=strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]),"data:"));if line==""||line=="[DONE]"{continue};var value any;if json.Unmarshal([]byte(line),&value)==nil{a,b:=findUsage(value);if a+b>0{return a,b}}};return 0,0}
func findUsage(value any)(int,int){if items,ok:=value.([]any);ok{for _,child:=range items{if a,b:=findUsage(child);a+b>0{return a,b}};return 0,0};m,ok:=value.(map[string]any);if !ok{return 0,0};if u,ok:=m["usage"].(map[string]any);ok{a:=intValue(defaultAny(u["inputTokens"],defaultAny(u["promptTokens"],defaultAny(u["prompt_tokens"],0))));b:=intValue(defaultAny(u["outputTokens"],defaultAny(u["completionTokens"],defaultAny(u["completion_tokens"],0))));if a+b>0{return a,b}};for _,child:=range m{if a,b:=findUsage(child);a+b>0{return a,b}};return 0,0}

func (s *Server) callAI(ctx context.Context,tenantID,userID,appID string,body map[string]any)(map[string]any,[2]int,error){
	config,err:=s.readAIConfig(ctx);if err!=nil{return nil,[2]int{},err};if config.Key==""{return nil,[2]int{},fmt.Errorf("企业尚未配置 AI 服务密钥")};if err=s.checkAIQuota(ctx,tenantID,userID);err!=nil{return nil,[2]int{},err}
	usage,err:=s.PB.Create(ctx,"ai_usage",map[string]any{"tenant_id":tenantID,"user_id":userID,"app_id":appID,"status":100,"input_tokens":0,"output_tokens":0});if err!=nil{return nil,[2]int{},err}
	body["model"]=config.Model;body["stream"]=false;encoded,_:=json.Marshal(body);target:=vercelGateway+"/v1/chat/completions";if config.Provider=="capi"{target=strings.TrimRight(config.BaseURL,"/")+"/chat/completions"}
	req,err:=http.NewRequestWithContext(ctx,http.MethodPost,target,bytes.NewReader(encoded));if err!=nil{return nil,[2]int{},err};req.Header.Set("Authorization","Bearer "+config.Key);req.Header.Set("Content-Type","application/json")
	resp,err:=http.DefaultClient.Do(req);if err!=nil{_,_=s.PB.Update(context.Background(),"ai_usage",stringValue(usage["id"]),map[string]any{"status":502});return nil,[2]int{},err};defer resp.Body.Close()
	data,_:=io.ReadAll(io.LimitReader(resp.Body,8<<20));_=s.PB.Update(context.Background(),"ai_usage",stringValue(usage["id"]),map[string]any{"status":resp.StatusCode})
	if resp.StatusCode<200||resp.StatusCode>=300{return nil,[2]int{},fmt.Errorf("AI gateway returned %d: %s",resp.StatusCode,clip(string(data),500))}
	result:=map[string]any{};if json.Unmarshal(data,&result)!=nil{return nil,[2]int{},fmt.Errorf("AI gateway response is invalid")}
	a,b:=findUsage(result);if a+b>0{_,_=s.PB.Update(context.Background(),"ai_usage",stringValue(usage["id"]),map[string]any{"input_tokens":a,"output_tokens":b})}
	return result,[2]int{a,b},nil
}
