package workflowy

import "context"

type Client interface {
	GetItem(ctx context.Context, itemID string) (*Item, error)
	ListChildren(ctx context.Context, itemID string) (*ListChildrenResponse, error)
	ListChildrenRecursive(ctx context.Context, itemID string) (*ListChildrenResponse, error)
	ListChildrenRecursiveWithDepth(ctx context.Context, itemID string, depth int) (*ListChildrenResponse, error)
	CreateNode(ctx context.Context, req *CreateNodeRequest) (*CreateNodeResponse, error)
	UpdateNode(ctx context.Context, itemID string, req *UpdateNodeRequest) (*UpdateNodeResponse, error)
	MoveNode(ctx context.Context, itemID string, req *MoveNodeRequest) (*MoveNodeResponse, error)
	CompleteNode(ctx context.Context, itemID string) (*UpdateNodeResponse, error)
	UncompleteNode(ctx context.Context, itemID string) (*UpdateNodeResponse, error)
	DeleteNode(ctx context.Context, itemID string) (*UpdateNodeResponse, error)
	ExportNodesWithCache(ctx context.Context, forceRefresh bool) (*ExportNodesResponse, error)
	ListTargets(ctx context.Context) (*ListTargetsResponse, error)
}

type BackupProvider interface {
	ReadBackupFile(filename string) ([]*Item, error)
	ReadLatestBackup() ([]*Item, error)
}

type FileBackupProvider struct{}

func (p *FileBackupProvider) ReadBackupFile(filename string) ([]*Item, error) {
	return ReadBackupFile(filename)
}

func (p *FileBackupProvider) ReadLatestBackup() ([]*Item, error) {
	return ReadLatestBackup()
}

var DefaultBackupProvider BackupProvider = &FileBackupProvider{}
