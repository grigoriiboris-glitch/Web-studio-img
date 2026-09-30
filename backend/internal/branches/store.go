package branches

import (
    "context"
    "database/sql"
    "encoding/json"
    "errors"
    "fmt"
    "strings"

    "github.com/google/uuid"
    "github.com/oleg3190/Web-studio-img/backend/internal/iterations"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) (*Store, error) {
    if db == nil { return nil, errors.New("branch store requires database") }
    return &Store{db: db}, nil
}

func (s *Store) List(ctx context.Context, userID, projectID uuid.UUID) ([]Branch, error) {
    if err := s.ownedProject(ctx,userID,projectID); err != nil { return nil,err }
    rows,err:=s.db.QueryContext(ctx, `SELECT id,project_id,name,parent_iteration_id,status,created_by,created_at,updated_at FROM branches WHERE project_id=$1 ORDER BY created_at ASC,id ASC`,projectID)
    if err!=nil{return nil,fmt.Errorf("list branches: %w",err)};defer func() { _ = rows.Close() }()
    out:=[]Branch{}
    for rows.Next(){var b Branch;if err:=rows.Scan(&b.ID,&b.ProjectID,&b.Name,&b.ParentIterationID,&b.Status,&b.CreatedBy,&b.CreatedAt,&b.UpdatedAt);err!=nil{return nil,err};out=append(out,b)}
    return out,rows.Err()
}

func (s *Store) Create(ctx context.Context,userID,projectID uuid.UUID,parentID *uuid.UUID,name string)(Branch,error){
    if err:=ValidateName(name);err!=nil{return Branch{},err}
    if err:=s.ownedProject(ctx,userID,projectID);err!=nil{return Branch{},err}
    if parentID!=nil{
        var pProject uuid.UUID
        if err:=s.db.QueryRowContext(ctx,`SELECT project_id FROM iterations WHERE id=$1`,*parentID).Scan(&pProject);errors.Is(err,sql.ErrNoRows){return Branch{},ErrBranchNotFound}else if err!=nil{return Branch{},err}else if pProject!=projectID{return Branch{},ErrBranchNotFound}
    }
    var b Branch
    err:=s.db.QueryRowContext(ctx,`INSERT INTO branches(project_id,name,parent_iteration_id,created_by) VALUES($1,$2,$3,$4) RETURNING id,project_id,name,parent_iteration_id,status,created_by,created_at,updated_at`,projectID,strings.TrimSpace(name),parentID,userID).Scan(&b.ID,&b.ProjectID,&b.Name,&b.ParentIterationID,&b.Status,&b.CreatedBy,&b.CreatedAt,&b.UpdatedAt)
    if err!=nil{return Branch{},fmt.Errorf("create branch: %w",err)}
    return b,nil
}

func (s *Store) Update(ctx context.Context,userID,projectID,branchID uuid.UUID,name string,status Status)(Branch,error){
    if err:=ValidateName(name);err!=nil{return Branch{},err};if err:=ValidateStatus(status);err!=nil{return Branch{},err}
    if err:=s.ownedProject(ctx,userID,projectID);err!=nil{return Branch{},err}
    var b Branch
    err:=s.db.QueryRowContext(ctx,`UPDATE branches SET name=$1,status=$2,updated_at=now() WHERE id=$3 AND project_id=$4 RETURNING id,project_id,name,parent_iteration_id,status,created_by,created_at,updated_at`,strings.TrimSpace(name),status,branchID,projectID).Scan(&b.ID,&b.ProjectID,&b.Name,&b.ParentIterationID,&b.Status,&b.CreatedBy,&b.CreatedAt,&b.UpdatedAt)
    if errors.Is(err,sql.ErrNoRows){return Branch{},ErrBranchNotFound};if err!=nil{return Branch{},fmt.Errorf("update branch: %w",err)}
    return b,nil
}

func (s *Store) Compare(ctx context.Context,userID,projectID,aID,bID uuid.UUID)([]CompareItem,error){
    if aID==bID{return nil,ErrMergeConflict};if err:=s.ownedProject(ctx,userID,projectID);err!=nil{return nil,err}
    a,err:=s.latestDecisions(ctx,projectID,aID);if err!=nil{return nil,err};b,err:=s.latestDecisions(ctx,projectID,bID);if err!=nil{return nil,err}
    out:=make([]CompareItem,0,len(decisionDimensions))
    for _,d:=range decisionDimensions{av,aok:=a[d];bv,bok:=b[d];different:=aok!=bok || (aok && stringify(av)!=stringify(bv));out=append(out,CompareItem{Dimension:d,Source:av,Target:bv,Different:different})}
    return out,nil
}

func (s *Store) Merge(ctx context.Context,userID,projectID,targetID uuid.UUID,req MergeRequest)(iterations.Iteration,error){
    if len(req.Sources)==0{return iterations.Iteration{},ErrMergeConflict}
    if err:=s.ownedProject(ctx,userID,projectID);err!=nil{return iterations.Iteration{},err}
    target,err:=s.branch(ctx,projectID,targetID);if err!=nil{return iterations.Iteration{},err}
    if target.Status==StatusArchived{return iterations.Iteration{},ErrBranchArchived}

    sourceValues:=map[string]map[uuid.UUID]any{}
    for _,sourceID:=range req.Sources{
        if sourceID==targetID{return iterations.Iteration{},ErrMergeConflict}
        b,berr:=s.branch(ctx,projectID,sourceID);if berr!=nil{return iterations.Iteration{},berr}
        if b.Status==StatusArchived{return iterations.Iteration{},ErrBranchArchived}
        vals,verr:=s.latestDecisions(ctx,projectID,sourceID);if verr!=nil{return iterations.Iteration{},verr}
        for dimension,value:=range vals{
            if !IsDecisionDimension(dimension){return iterations.Iteration{},ErrMergeConflict}
            if sourceValues[dimension]==nil{sourceValues[dimension]=map[uuid.UUID]any{}}
            sourceValues[dimension][sourceID]=value
        }
    }
    targetVals,err:=s.latestDecisions(ctx,projectID,targetID);if err!=nil{return iterations.Iteration{},err}
    merged:=map[string]any{}
    for k,v:=range targetVals{merged[k]=v}

    for dimension,values:=range sourceValues{
        unique:=map[string]bool{}
        for _,value:=range values{unique[stringify(value)]=true}
        if len(unique)>1{
            selection,ok:=req.Decisions[dimension]
            if !ok{return iterations.Iteration{},ErrMergeConflict}
            value,exists:=values[selection.SourceBranchID]
            if !exists{return iterations.Iteration{},ErrMergeConflict}
            merged[dimension]=value
            continue
        }
        for _,value:=range values{merged[dimension]=value;break}
    }
    for dimension,selection:=range req.Decisions{
        if !IsDecisionDimension(dimension)||selection.SourceBranchID==uuid.Nil{return iterations.Iteration{},ErrMergeConflict}
        values:=sourceValues[dimension];value,ok:=values[selection.SourceBranchID];if !ok{return iterations.Iteration{},ErrMergeConflict}
        merged[dimension]=value
    }

    parentID:=(*uuid.UUID)(nil)
    var latest uuid.UUID
    if err:=s.db.QueryRowContext(ctx,`SELECT id FROM iterations WHERE branch_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1`,targetID).Scan(&latest);err==nil{parentID=&latest}else if !errors.Is(err,sql.ErrNoRows){return iterations.Iteration{},err}
    raw,_:=json.Marshal(merged);sourcesRaw,_:=json.Marshal(req.Sources)
    var item iterations.Iteration
    err=s.db.QueryRowContext(ctx,`INSERT INTO iterations(project_id,branch_id,parent_iteration_id,type,title,description,decisions,merge_sources) VALUES($1,$2,$3,'selection','Merge decisions',NULL,$4,$5) RETURNING id,project_id,branch_id,parent_iteration_id,type,title,description,decisions,merge_sources,created_at`,projectID,targetID,parentID,raw,sourcesRaw).Scan(&item.ID,&item.ProjectID,&item.BranchID,&item.ParentIterationID,&item.Type,&item.Title,&item.Description,&item.Decisions,&item.MergeSources,&item.CreatedAt)
    if err!=nil{return iterations.Iteration{},fmt.Errorf("merge iteration: %w",err)}
    return item,nil
}

func validateConflicts(sources []uuid.UUID,merged,target map[string]any) error {
    if len(sources)<2{return nil}
    seen:=map[string]string{}
    for _,id:=range sources{_ = id}
    for k,v:=range merged{sv:=stringify(v);if prev,ok:=seen[k];ok&&prev!=sv{return ErrMergeConflict};seen[k]=sv}
    _=target
    return nil
}

func (s *Store) latestDecisions(ctx context.Context,projectID,branchID uuid.UUID)(map[string]any,error){
    if _,err:=s.branch(ctx,projectID,branchID);err!=nil{return nil,err}
    var raw []byte
    err:=s.db.QueryRowContext(ctx,`SELECT decisions FROM iterations WHERE project_id=$1 AND branch_id=$2 ORDER BY created_at DESC,id DESC LIMIT 1`,projectID,branchID).Scan(&raw)
    if errors.Is(err,sql.ErrNoRows){return map[string]any{},nil};if err!=nil{return nil,err}
    out:=map[string]any{};if len(raw)>0{if err:=json.Unmarshal(raw,&out);err!=nil{return nil,err}};return out,nil
}
func (s *Store) branch(ctx context.Context,projectID,id uuid.UUID)(Branch,error){
    var b Branch;err:=s.db.QueryRowContext(ctx,`SELECT id,project_id,name,parent_iteration_id,status,created_by,created_at,updated_at FROM branches WHERE id=$1 AND project_id=$2`,id,projectID).Scan(&b.ID,&b.ProjectID,&b.Name,&b.ParentIterationID,&b.Status,&b.CreatedBy,&b.CreatedAt,&b.UpdatedAt);if errors.Is(err,sql.ErrNoRows){return Branch{},ErrBranchNotFound};return b,err
}
func (s *Store) ownedProject(ctx context.Context,userID,projectID uuid.UUID)error{
    var ok bool;if err:=s.db.QueryRowContext(ctx,`SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status<>'deleted')`,projectID,userID).Scan(&ok);err!=nil{return err};if !ok{return ErrBranchNotFound};return nil
}
func stringify(v any)string{b,_:=json.Marshal(v);return string(b)}
