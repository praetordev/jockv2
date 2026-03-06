package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/daearol/jockv2/backend/internal/dsl"
	"github.com/daearol/jockv2/backend/internal/git"
	pb "github.com/daearol/jockv2/backend/internal/proto"
	"github.com/daearol/jockv2/backend/internal/tasks"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GitServer implements the GitService gRPC service.
type GitServer struct {
	pb.UnimplementedGitServiceServer
	cache dsl.CommitCache
}

// New creates a new GitServer with an optional cache.
func New(cache dsl.CommitCache) *GitServer {
	return &GitServer{cache: cache}
}

func (s *GitServer) ListCommits(_ context.Context, req *pb.ListCommitsRequest) (*pb.ListCommitsResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}

	limit := int(req.Limit)
	if limit == 0 {
		limit = 100
	}

	var filter *git.CommitFilter
	if req.AuthorPattern != "" || req.GrepPattern != "" || req.AfterDate != "" || req.BeforeDate != "" || req.PathPattern != "" {
		filter = &git.CommitFilter{
			AuthorPattern: req.AuthorPattern,
			GrepPattern:   req.GrepPattern,
			AfterDate:     req.AfterDate,
			BeforeDate:    req.BeforeDate,
			PathPattern:   req.PathPattern,
		}
	}

	rawCommits, err := git.ListRawCommits(req.RepoPath, limit, int(req.Skip), req.Branch, filter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "git error: %v", err)
	}

	graphData := git.ComputeGraph(rawCommits)

	var commits []*pb.Commit
	for _, rc := range rawCommits {
		branches, tags := git.ParseRefs(rc.Refs)
		gd := graphData[rc.Hash]

		var conns []*pb.GraphConnection
		for _, conn := range gd.Connections {
			conns = append(conns, &pb.GraphConnection{
				ToColumn: int32(conn.ToColumn),
				ToRow:    int32(conn.ToRow),
				Color:    conn.Color,
			})
		}

		shortHash := rc.Hash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}

		commits = append(commits, &pb.Commit{
			Hash:     shortHash,
			Message:  rc.Message,
			Author:   rc.Author,
			Date:     rc.Date,
			Branches: branches,
			Tags:     tags,
			Parents:  shortenHashes(rc.Parents),
			Graph: &pb.CommitGraph{
				Color:       gd.Color,
				Column:      int32(gd.Column),
				Connections: conns,
			},
		})
	}

	return &pb.ListCommitsResponse{Commits: commits}, nil
}

func (s *GitServer) ListBranches(_ context.Context, req *pb.ListBranchesRequest) (*pb.ListBranchesResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}

	branchInfos, err := git.ListBranches(req.RepoPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "git error: %v", err)
	}

	var branches []*pb.Branch
	for _, bi := range branchInfos {
		branches = append(branches, &pb.Branch{
			Name:      bi.Name,
			IsCurrent: bi.IsCurrent,
			Remote:    bi.Remote,
			Ahead:     int32(bi.Ahead),
			Behind:    int32(bi.Behind),
		})
	}

	return &pb.ListBranchesResponse{Branches: branches}, nil
}

func (s *GitServer) GetCommitDetails(_ context.Context, req *pb.GetCommitDetailsRequest) (*pb.GetCommitDetailsResponse, error) {
	if req.RepoPath == "" || req.Hash == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and hash required")
	}

	// Get file changes for the commit
	fileInfos, err := git.GetCommitDiffStat(req.RepoPath, req.Hash)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "git error: %v", err)
	}

	// Get the full patch to split into per-file patches
	fullPatch, _ := git.GetCommitPatch(req.RepoPath, req.Hash)
	patchMap := splitPatchByFile(fullPatch)

	var fileChanges []*pb.FileChange
	for _, fi := range fileInfos {
		fileChanges = append(fileChanges, &pb.FileChange{
			Path:      fi.Path,
			Status:    fi.Status,
			Additions: int32(fi.Additions),
			Deletions: int32(fi.Deletions),
			Patch:     patchMap[fi.Path],
		})
	}

	return &pb.GetCommitDetailsResponse{
		FileChanges: fileChanges,
	}, nil
}

func (s *GitServer) GetFileDiff(_ context.Context, req *pb.GetFileDiffRequest) (*pb.GetFileDiffResponse, error) {
	if req.RepoPath == "" || req.Hash == "" || req.FilePath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path, hash, and file_path required")
	}

	patch, err := git.GetFilePatch(req.RepoPath, req.Hash, req.FilePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "git error: %v", err)
	}

	return &pb.GetFileDiffResponse{Patch: patch}, nil
}

func (s *GitServer) GetStatus(_ context.Context, req *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}

	staged, unstaged, untracked, unmerged, err := git.GetStatus(req.RepoPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "git error: %v", err)
	}

	toProto := func(infos []git.FileChangeInfo) []*pb.FileChange {
		var result []*pb.FileChange
		for _, fi := range infos {
			result = append(result, &pb.FileChange{
				Path:      fi.Path,
				Status:    fi.Status,
				Additions: int32(fi.Additions),
				Deletions: int32(fi.Deletions),
			})
		}
		return result
	}

	return &pb.GetStatusResponse{
		Staged:    toProto(staged),
		Unstaged:  toProto(unstaged),
		Untracked: untracked,
		Unmerged:  toProto(unmerged),
	}, nil
}

func (s *GitServer) Pull(_ context.Context, req *pb.PullRequest) (*pb.PullResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}

	result, err := git.Pull(req.RepoPath, req.Remote, req.Branch)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "git pull: %v", err)
	}

	return &pb.PullResponse{
		Summary: result.Summary,
		Updated: int32(result.Updated),
	}, nil
}

func (s *GitServer) ListRemotes(_ context.Context, req *pb.ListRemotesRequest) (*pb.ListRemotesResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}

	remotes, err := git.ListRemotes(req.RepoPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "git error: %v", err)
	}

	var pbRemotes []*pb.RemoteInfo
	for _, r := range remotes {
		pbRemotes = append(pbRemotes, &pb.RemoteInfo{
			Name: r.Name,
			Url:  r.URL,
		})
	}

	return &pb.ListRemotesResponse{Remotes: pbRemotes}, nil
}

func (s *GitServer) ListRemoteBranches(_ context.Context, req *pb.ListRemoteBranchesRequest) (*pb.ListRemoteBranchesResponse, error) {
	if req.RepoPath == "" || req.Remote == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and remote required")
	}

	branches, err := git.ListRemoteBranches(req.RepoPath, req.Remote)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "git error: %v", err)
	}

	return &pb.ListRemoteBranchesResponse{Branches: branches}, nil
}

func (s *GitServer) ExecuteDSL(_ context.Context, req *pb.ExecuteDSLRequest) (*pb.ExecuteDSLResponse, error) {
	if req.RepoPath == "" || req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and query required")
	}

	pipeline, err := dsl.Parse(req.Query)
	if err != nil {
		return &pb.ExecuteDSLResponse{
			ResultKind: "error",
			Error:      err.Error(),
		}, nil
	}

	evalCtx := &dsl.EvalContext{
		RepoPath: req.RepoPath,
		DryRun:   req.DryRun,
		Ctx:      context.Background(),
		Cache:    s.cache,
	}
	result, err := dsl.Evaluate(evalCtx, pipeline)
	if err != nil {
		return &pb.ExecuteDSLResponse{
			ResultKind: "error",
			Error:      err.Error(),
		}, nil
	}

	return dslResultToProto(result), nil
}

func (s *GitServer) DSLAutoComplete(_ context.Context, req *pb.DSLAutoCompleteRequest) (*pb.DSLAutoCompleteResponse, error) {
	suggestions := dsl.AutoComplete(req.RepoPath, req.PartialQuery, int(req.CursorPosition))

	var pbSuggestions []*pb.DSLSuggestion
	for _, sg := range suggestions {
		pbSuggestions = append(pbSuggestions, &pb.DSLSuggestion{
			Text:        sg.Text,
			Kind:        sg.Kind,
			Description: sg.Description,
		})
	}

	return &pb.DSLAutoCompleteResponse{Suggestions: pbSuggestions}, nil
}

// --- Stage / Unstage / Commit ---

func (s *GitServer) StageFiles(_ context.Context, req *pb.StageFilesRequest) (*pb.StageFilesResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	if err := git.StageFiles(req.RepoPath, req.Paths); err != nil {
		return nil, status.Errorf(codes.Internal, "stage: %v", err)
	}
	return &pb.StageFilesResponse{Success: true}, nil
}

func (s *GitServer) UnstageFiles(_ context.Context, req *pb.UnstageFilesRequest) (*pb.UnstageFilesResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	if err := git.UnstageFiles(req.RepoPath, req.Paths); err != nil {
		return nil, status.Errorf(codes.Internal, "unstage: %v", err)
	}
	return &pb.UnstageFilesResponse{Success: true}, nil
}

func (s *GitServer) CreateCommit(_ context.Context, req *pb.CreateCommitRequest) (*pb.CreateCommitResponse, error) {
	if req.RepoPath == "" || req.Message == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and message required")
	}
	result, err := git.CreateCommit(req.RepoPath, req.Message, req.Amend)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "commit: %v", err)
	}
	return &pb.CreateCommitResponse{Hash: result.Hash}, nil
}

func (s *GitServer) GetWorkingDiff(_ context.Context, req *pb.GetWorkingDiffRequest) (*pb.GetWorkingDiffResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	patch, err := git.GetWorkingDiff(req.RepoPath, req.FilePath, req.Staged)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "working diff: %v", err)
	}
	return &pb.GetWorkingDiffResponse{Patch: patch}, nil
}

// --- Push ---

func (s *GitServer) Push(_ context.Context, req *pb.PushRequest) (*pb.PushResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	result, err := git.Push(req.RepoPath, req.Remote, req.Branch, req.Force, req.SetUpstream)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "push: %v", err)
	}
	return &pb.PushResponse{Summary: result.Summary, Success: result.Success}, nil
}

// --- Merge ---

func (s *GitServer) Merge(_ context.Context, req *pb.MergeRequest) (*pb.MergeResponse, error) {
	if req.RepoPath == "" || req.Branch == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and branch required")
	}
	result, err := git.Merge(req.RepoPath, req.Branch, req.NoFf)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "merge: %v", err)
	}
	return &pb.MergeResponse{
		Summary:       result.Summary,
		Success:       result.Success,
		HasConflicts:  result.HasConflicts,
		ConflictFiles: result.ConflictFiles,
	}, nil
}

// --- Branch Management ---

func (s *GitServer) CreateBranch(_ context.Context, req *pb.CreateBranchRequest) (*pb.CreateBranchResponse, error) {
	if req.RepoPath == "" || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and name required")
	}
	if err := git.CreateBranch(req.RepoPath, req.Name, req.StartPoint, req.Checkout); err != nil {
		return nil, status.Errorf(codes.Internal, "create branch: %v", err)
	}
	return &pb.CreateBranchResponse{Success: true, Name: req.Name}, nil
}

func (s *GitServer) DeleteBranch(_ context.Context, req *pb.DeleteBranchRequest) (*pb.DeleteBranchResponse, error) {
	if req.RepoPath == "" || req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and name required")
	}
	if err := git.DeleteBranch(req.RepoPath, req.Name, req.Force); err != nil {
		return nil, status.Errorf(codes.Internal, "delete branch: %v", err)
	}
	return &pb.DeleteBranchResponse{Success: true}, nil
}

// --- Conflict Resolution ---

func (s *GitServer) GetConflictDetails(_ context.Context, req *pb.GetConflictDetailsRequest) (*pb.GetConflictDetailsResponse, error) {
	if req.RepoPath == "" || req.FilePath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and file_path required")
	}
	detail, err := git.GetConflictDetails(req.RepoPath, req.FilePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "conflict details: %v", err)
	}
	return &pb.GetConflictDetailsResponse{
		Path:          detail.Path,
		OursContent:   detail.OursContent,
		TheirsContent: detail.TheirsContent,
		RawContent:    detail.RawContent,
	}, nil
}

func (s *GitServer) ResolveConflict(_ context.Context, req *pb.ResolveConflictRequest) (*pb.ResolveConflictResponse, error) {
	if req.RepoPath == "" || req.FilePath == "" || req.Strategy == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path, file_path, and strategy required")
	}
	if err := git.ResolveConflict(req.RepoPath, req.FilePath, req.Strategy); err != nil {
		return nil, status.Errorf(codes.Internal, "resolve conflict: %v", err)
	}
	return &pb.ResolveConflictResponse{Success: true}, nil
}

func (s *GitServer) AbortMerge(_ context.Context, req *pb.AbortMergeRequest) (*pb.AbortMergeResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	if err := git.AbortMerge(req.RepoPath); err != nil {
		return nil, status.Errorf(codes.Internal, "abort merge: %v", err)
	}
	return &pb.AbortMergeResponse{Success: true}, nil
}

func (s *GitServer) IsMerging(_ context.Context, req *pb.IsMergingRequest) (*pb.IsMergingResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	return &pb.IsMergingResponse{IsMerging: git.IsMerging(req.RepoPath)}, nil
}

// --- Cherry-Pick / Revert ---

func (s *GitServer) CherryPick(_ context.Context, req *pb.CherryPickRequest) (*pb.CherryPickResponse, error) {
	if req.RepoPath == "" || req.CommitHash == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and commit_hash required")
	}
	result, err := git.CherryPick(req.RepoPath, req.CommitHash)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cherry-pick: %v", err)
	}
	return &pb.CherryPickResponse{
		Success:       result.Success,
		Summary:       result.Summary,
		HasConflicts:  result.HasConflicts,
		ConflictFiles: result.ConflictFiles,
	}, nil
}

func (s *GitServer) Revert(_ context.Context, req *pb.RevertRequest) (*pb.RevertResponse, error) {
	if req.RepoPath == "" || req.CommitHash == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and commit_hash required")
	}
	result, err := git.Revert(req.RepoPath, req.CommitHash)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "revert: %v", err)
	}
	return &pb.RevertResponse{
		Success:       result.Success,
		Summary:       result.Summary,
		HasConflicts:  result.HasConflicts,
		ConflictFiles: result.ConflictFiles,
	}, nil
}

// --- Tag Management ---

func (s *GitServer) ListTags(_ context.Context, req *pb.ListTagsRequest) (*pb.ListTagsResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	tags, err := git.ListTags(req.RepoPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list tags: %v", err)
	}
	var pbTags []*pb.TagInfo
	for _, t := range tags {
		pbTags = append(pbTags, &pb.TagInfo{
			Name:        t.Name,
			Hash:        t.Hash,
			Date:        t.Date,
			Message:     t.Message,
			IsAnnotated: t.IsAnnotated,
		})
	}
	return &pb.ListTagsResponse{Tags: pbTags}, nil
}

func (s *GitServer) CreateTag(_ context.Context, req *pb.CreateTagRequest) (*pb.CreateTagResponse, error) {
	if req.RepoPath == "" || req.TagName == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and tag_name required")
	}
	if err := git.CreateTag(req.RepoPath, req.TagName, req.CommitHash, req.Message); err != nil {
		return nil, status.Errorf(codes.Internal, "create tag: %v", err)
	}
	return &pb.CreateTagResponse{Success: true}, nil
}

func (s *GitServer) DeleteTag(_ context.Context, req *pb.DeleteTagRequest) (*pb.DeleteTagResponse, error) {
	if req.RepoPath == "" || req.TagName == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and tag_name required")
	}
	if err := git.DeleteTag(req.RepoPath, req.TagName); err != nil {
		return nil, status.Errorf(codes.Internal, "delete tag: %v", err)
	}
	return &pb.DeleteTagResponse{Success: true}, nil
}

func (s *GitServer) PushTag(_ context.Context, req *pb.PushTagRequest) (*pb.PushTagResponse, error) {
	if req.RepoPath == "" || req.TagName == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and tag_name required")
	}
	if err := git.PushTag(req.RepoPath, req.Remote, req.TagName); err != nil {
		return nil, status.Errorf(codes.Internal, "push tag: %v", err)
	}
	return &pb.PushTagResponse{Success: true}, nil
}

// --- Stash ---

func (s *GitServer) ListStashes(_ context.Context, req *pb.ListStashesRequest) (*pb.ListStashesResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	stashes, err := git.ListStashes(req.RepoPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list stashes: %v", err)
	}
	var pbStashes []*pb.StashEntry
	for _, s := range stashes {
		pbStashes = append(pbStashes, &pb.StashEntry{
			Index:   int32(s.Index),
			Message: s.Message,
			Branch:  s.Branch,
			Date:    s.Date,
		})
	}
	return &pb.ListStashesResponse{Stashes: pbStashes}, nil
}

func (s *GitServer) CreateStash(_ context.Context, req *pb.CreateStashRequest) (*pb.CreateStashResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	if err := git.CreateStash(req.RepoPath, req.Message, req.IncludeUntracked); err != nil {
		return nil, status.Errorf(codes.Internal, "create stash: %v", err)
	}
	return &pb.CreateStashResponse{Success: true}, nil
}

func (s *GitServer) ApplyStash(_ context.Context, req *pb.ApplyStashRequest) (*pb.ApplyStashResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	summary, err := git.ApplyStash(req.RepoPath, int(req.Index))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "apply stash: %v", err)
	}
	return &pb.ApplyStashResponse{Success: true, Summary: summary}, nil
}

func (s *GitServer) PopStash(_ context.Context, req *pb.PopStashRequest) (*pb.PopStashResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	summary, err := git.PopStash(req.RepoPath, int(req.Index))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "pop stash: %v", err)
	}
	return &pb.PopStashResponse{Success: true, Summary: summary}, nil
}

func (s *GitServer) DropStash(_ context.Context, req *pb.DropStashRequest) (*pb.DropStashResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	if err := git.DropStash(req.RepoPath, int(req.Index)); err != nil {
		return nil, status.Errorf(codes.Internal, "drop stash: %v", err)
	}
	return &pb.DropStashResponse{Success: true}, nil
}

func (s *GitServer) ShowStash(_ context.Context, req *pb.ShowStashRequest) (*pb.ShowStashResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	patch, err := git.ShowStash(req.RepoPath, int(req.Index))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "show stash: %v", err)
	}
	return &pb.ShowStashResponse{Patch: patch}, nil
}

// --- Helpers ---

func dslResultToProto(r *dsl.Result) *pb.ExecuteDSLResponse {
	resp := &pb.ExecuteDSLResponse{ResultKind: r.Kind}

	switch r.Kind {
	case "commits":
		// Convert DSL commits to RawCommits for graph computation
		rawCommits := make([]git.RawCommit, len(r.Commits))
		for i, c := range r.Commits {
			var refs []string
			for _, b := range c.Branches {
				refs = append(refs, b)
			}
			for _, t := range c.Tags {
				refs = append(refs, "tag: "+t)
			}
			rawCommits[i] = git.RawCommit{
				Hash:    c.Hash,
				Message: c.Message,
				Author:  c.Author,
				Date:    c.Date,
				Parents: c.Parents,
				Refs:    strings.Join(refs, ", "),
			}
		}
		graphData := git.ComputeGraph(rawCommits)

		for _, c := range r.Commits {
			gd := graphData[c.Hash]
			var conns []*pb.GraphConnection
			for _, conn := range gd.Connections {
				conns = append(conns, &pb.GraphConnection{
					ToColumn: int32(conn.ToColumn),
					ToRow:    int32(conn.ToRow),
					Color:    conn.Color,
				})
			}
			shortHash := c.Hash
			if len(shortHash) > 7 {
				shortHash = shortHash[:7]
			}
			resp.Commits = append(resp.Commits, &pb.DSLCommitResult{
				Hash:      shortHash,
				Message:   c.Message,
				Author:    c.Author,
				Date:      c.Date,
				Branches:  c.Branches,
				Tags:      c.Tags,
				Additions: int32(c.Additions),
				Deletions: int32(c.Deletions),
				Files:     c.Files,
				DateIso:   c.DateISO,
				Parents:   shortenHashes(c.Parents),
				Graph: &pb.CommitGraph{
					Color:       gd.Color,
					Column:      int32(gd.Column),
					Connections: conns,
				},
			})
		}
	case "branches":
		for _, b := range r.Branches {
			resp.Branches = append(resp.Branches, &pb.Branch{
				Name:      b.Name,
				IsCurrent: b.IsCurrent,
				Remote:    b.Remote,
			})
		}
	case "count":
		resp.Count = int32(r.Count)
	case "aggregate":
		for _, row := range r.Aggregates {
			resp.Aggregates = append(resp.Aggregates, &pb.DSLAggregateRow{
				Group: row.Group,
				Value: row.Value,
			})
		}
		resp.AggFunc = r.AggFunc
		resp.AggField = r.AggField
		resp.GroupField = r.GroupField
	case "files":
		// Render files as a formatted table for the frontend
		resp.ResultKind = "formatted"
		resp.FormatType = "table"
		var sb strings.Builder
		for _, f := range r.Files {
			sb.WriteString(fmt.Sprintf("%-40s  +%-5d  -%-5d  %d commits\n", f.Path, f.Additions, f.Deletions, f.Commits))
		}
		resp.FormattedOutput = strings.TrimRight(sb.String(), "\n")
	case "blame":
		resp.ResultKind = "formatted"
		resp.FormatType = "table"
		var sb strings.Builder
		for _, l := range r.BlameLines {
			sb.WriteString(fmt.Sprintf("%4d  %s  %-12s  %s  %s\n", l.LineNo, l.Hash[:7], l.Author, l.Date, l.Content))
		}
		resp.FormattedOutput = strings.TrimRight(sb.String(), "\n")
	case "stashes":
		resp.ResultKind = "formatted"
		resp.FormatType = "table"
		var sb strings.Builder
		for _, s := range r.Stashes {
			sb.WriteString(fmt.Sprintf("stash@{%d}  %-12s  %s  %s\n", s.Index, s.Branch, s.Date, s.Message))
		}
		if sb.Len() == 0 {
			sb.WriteString("(no stashes)")
		}
		resp.FormattedOutput = strings.TrimRight(sb.String(), "\n")
	case "tasks":
		resp.ResultKind = "formatted"
		resp.FormatType = "table"
		var sb strings.Builder
		priorities := []string{"", "low", "med", "high"}
		for _, t := range r.Tasks {
			pri := ""
			if t.Priority > 0 && t.Priority < len(priorities) {
				pri = priorities[t.Priority]
			}
			labels := ""
			if len(t.Labels) > 0 {
				labels = "[" + strings.Join(t.Labels, ",") + "]"
			}
			branch := ""
			if t.Branch != "" {
				branch = "→ " + t.Branch
			}
			sb.WriteString(fmt.Sprintf("%-5s %-12s %-4s %-40s %s %s\n", t.ID, t.Status, pri, t.Title, labels, branch))
		}
		if sb.Len() == 0 {
			sb.WriteString("(no tasks)")
		}
		resp.FormattedOutput = strings.TrimRight(sb.String(), "\n")
	case "formatted":
		resp.FormattedOutput = r.FormattedOutput
		resp.FormatType = r.FormatType
	case "action_report":
		if r.Report != nil {
			resp.ActionReport = &pb.DSLActionReport{
				Action:         r.Report.Action,
				AffectedHashes: r.Report.Affected,
				Success:        r.Report.Success,
				DryRun:         r.Report.DryRun,
				Description:    r.Report.Description,
				Errors:         r.Report.Errors,
			}
		}
	}

	return resp
}

// splitPatchByFile splits a unified diff into per-file patches.
func splitPatchByFile(fullPatch string) map[string]string {
	result := make(map[string]string)
	if fullPatch == "" {
		return result
	}

	sections := strings.Split(fullPatch, "diff --git ")
	for _, section := range sections[1:] { // skip first empty element
		lines := strings.SplitN(section, "\n", 2)
		if len(lines) < 2 {
			continue
		}

		// Parse "a/path b/path" header
		header := lines[0]
		parts := strings.SplitN(header, " ", 2)
		if len(parts) < 2 {
			continue
		}
		path := strings.TrimPrefix(parts[1], "b/")

		// The patch content is everything after the diff header, starting from @@
		content := lines[1]
		if idx := strings.Index(content, "@@"); idx != -1 {
			result[path] = strings.TrimSpace(content[idx:])
		}
	}
	return result
}

func (s *GitServer) Blame(ctx context.Context, req *pb.BlameRequest) (*pb.BlameResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	if req.FilePath == "" {
		return nil, status.Error(codes.InvalidArgument, "file_path required")
	}
	lines, err := git.Blame(req.RepoPath, req.FilePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	pbLines := make([]*pb.BlameLine, len(lines))
	for i, l := range lines {
		hash := l.Hash
		if len(hash) > 7 {
			hash = hash[:7]
		}
		pbLines[i] = &pb.BlameLine{
			Hash:    hash,
			Author:  l.Author,
			Date:    l.Date,
			LineNo:  int32(l.LineNo),
			Content: l.Content,
		}
	}
	return &pb.BlameResponse{Lines: pbLines}, nil
}

func shortenHashes(hashes []string) []string {
	short := make([]string, len(hashes))
	for i, h := range hashes {
		if len(h) > 7 {
			short[i] = h[:7]
		} else {
			short[i] = h
		}
	}
	return short
}

// --- Rebase ---

func (s *GitServer) Rebase(_ context.Context, req *pb.RebaseRequest) (*pb.RebaseResponse, error) {
	if req.RepoPath == "" || req.OntoBranch == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and onto_branch required")
	}
	result, err := git.Rebase(req.RepoPath, req.OntoBranch)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "rebase: %v", err)
	}
	return &pb.RebaseResponse{
		Success:       result.Success,
		Summary:       result.Summary,
		HasConflicts:  result.HasConflicts,
		ConflictFiles: result.ConflictFiles,
	}, nil
}

func (s *GitServer) AbortRebase(_ context.Context, req *pb.AbortRebaseRequest) (*pb.AbortRebaseResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	if err := git.AbortRebase(req.RepoPath); err != nil {
		return nil, status.Errorf(codes.Internal, "abort rebase: %v", err)
	}
	return &pb.AbortRebaseResponse{Success: true}, nil
}

func (s *GitServer) ContinueRebase(_ context.Context, req *pb.ContinueRebaseRequest) (*pb.ContinueRebaseResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	result, err := git.ContinueRebase(req.RepoPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "continue rebase: %v", err)
	}
	return &pb.ContinueRebaseResponse{
		Success:       result.Success,
		Summary:       result.Summary,
		HasConflicts:  result.HasConflicts,
		ConflictFiles: result.ConflictFiles,
	}, nil
}

func (s *GitServer) IsRebasing(_ context.Context, req *pb.IsRebasingRequest) (*pb.IsRebasingResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	return &pb.IsRebasingResponse{IsRebasing: git.IsRebasing(req.RepoPath)}, nil
}

// --- Interactive Rebase ---

func (s *GitServer) InteractiveRebase(_ context.Context, req *pb.InteractiveRebaseRequest) (*pb.InteractiveRebaseResponse, error) {
	if req.RepoPath == "" || req.BaseCommit == "" || len(req.Entries) == 0 {
		return nil, status.Error(codes.InvalidArgument, "repo_path, base_commit, and entries required")
	}
	entries := make([]git.RebaseTodoEntry, len(req.Entries))
	for i, e := range req.Entries {
		entries[i] = git.RebaseTodoEntry{Action: e.Action, Hash: e.Hash, Message: e.Message}
	}
	result, err := git.InteractiveRebase(req.RepoPath, req.BaseCommit, entries)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "interactive rebase: %v", err)
	}
	return &pb.InteractiveRebaseResponse{
		Success:       result.Success,
		Summary:       result.Summary,
		HasConflicts:  result.HasConflicts,
		ConflictFiles: result.ConflictFiles,
	}, nil
}

func (s *GitServer) GetRebaseTodo(_ context.Context, req *pb.GetRebaseTodoRequest) (*pb.GetRebaseTodoResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	entries, err := git.GetRebaseTodo(req.RepoPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get rebase todo: %v", err)
	}
	pbEntries := make([]*pb.RebaseTodoEntry, len(entries))
	for i, e := range entries {
		pbEntries[i] = &pb.RebaseTodoEntry{Action: e.Action, Hash: e.Hash, Message: e.Message}
	}
	return &pb.GetRebaseTodoResponse{Entries: pbEntries}, nil
}

// --- Reflog ---

func (s *GitServer) ListReflog(_ context.Context, req *pb.ListReflogRequest) (*pb.ListReflogResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	entries, err := git.ListReflog(req.RepoPath, int(req.Limit))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "reflog: %v", err)
	}
	pbEntries := make([]*pb.ReflogEntry, len(entries))
	for i, e := range entries {
		pbEntries[i] = &pb.ReflogEntry{Hash: e.Hash, Action: e.Action, Message: e.Message, Date: e.Date}
	}
	return &pb.ListReflogResponse{Entries: pbEntries}, nil
}

// --- Tasks ---

func taskToProto(t *tasks.Task) *pb.Task {
	return &pb.Task{
		Id:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status,
		Labels:      t.Labels,
		Branch:      t.Branch,
		Commits:     t.Commits,
		Created:     t.Created,
		Updated:     t.Updated,
		Priority:    int32(t.Priority),
	}
}

func (s *GitServer) ListTasks(_ context.Context, req *pb.ListTasksRequest) (*pb.ListTasksResponse, error) {
	if req.RepoPath == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path required")
	}
	taskList, err := tasks.List(req.RepoPath, req.StatusFilter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list tasks: %v", err)
	}
	var pbTasks []*pb.Task
	for _, t := range taskList {
		pbTasks = append(pbTasks, taskToProto(&t))
	}
	return &pb.ListTasksResponse{Tasks: pbTasks}, nil
}

func (s *GitServer) CreateTask(_ context.Context, req *pb.CreateTaskRequest) (*pb.CreateTaskResponse, error) {
	if req.RepoPath == "" || req.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and title required")
	}
	t, err := tasks.Create(req.RepoPath, req.Title, req.Description, req.Labels, int(req.Priority))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create task: %v", err)
	}
	return &pb.CreateTaskResponse{Task: taskToProto(t)}, nil
}

func (s *GitServer) UpdateTask(_ context.Context, req *pb.UpdateTaskRequest) (*pb.UpdateTaskResponse, error) {
	if req.RepoPath == "" || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and id required")
	}
	t, err := tasks.Update(req.RepoPath, req.Id, req.Title, req.Description, req.Status, req.Labels, req.Branch, int(req.Priority))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update task: %v", err)
	}
	return &pb.UpdateTaskResponse{Task: taskToProto(t)}, nil
}

func (s *GitServer) DeleteTask(_ context.Context, req *pb.DeleteTaskRequest) (*pb.DeleteTaskResponse, error) {
	if req.RepoPath == "" || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and id required")
	}
	if err := tasks.Delete(req.RepoPath, req.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "delete task: %v", err)
	}
	return &pb.DeleteTaskResponse{Success: true}, nil
}

func (s *GitServer) StartTask(_ context.Context, req *pb.StartTaskRequest) (*pb.StartTaskResponse, error) {
	if req.RepoPath == "" || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "repo_path and id required")
	}

	// Get the task first
	t, err := tasks.Get(req.RepoPath, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get task: %v", err)
	}

	branchName := ""
	if req.CreateBranch {
		branchName = fmt.Sprintf("task/%s-%s", t.ID, slugifyForBranch(t.Title))
		if err := git.CreateBranch(req.RepoPath, branchName, "", true); err != nil {
			return nil, status.Errorf(codes.Internal, "create branch: %v", err)
		}
	}

	// Update task status and branch
	t, err = tasks.Update(req.RepoPath, req.Id, "", "", "in-progress", nil, branchName, -1)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update task: %v", err)
	}

	return &pb.StartTaskResponse{Task: taskToProto(t), BranchName: branchName}, nil
}

func slugifyForBranch(title string) string {
	s := strings.ToLower(title)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, s)
	// Collapse multiple dashes
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if len(s) > 30 {
		s = s[:30]
		s = strings.TrimRight(s, "-")
	}
	return s
}
