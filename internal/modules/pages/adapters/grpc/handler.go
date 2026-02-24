package grpcadapter

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	jnats "github.com/nats-io/nats.go"
	"github.com/reggieanim/jot/internal/modules/pages/app"
	"github.com/reggieanim/jot/internal/modules/pages/domain"
	"github.com/reggieanim/jot/internal/shared/errs"
	pagesv1 "github.com/reggieanim/jot/proto/jot/pages/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pagesv1.UnimplementedPagesServer
	pagesv1.UnimplementedPagesRealtimeServer
	service *app.Service
	conn    *jnats.Conn
	subject string
	logger  *zap.Logger
}

type pageEvent struct {
	Type      string      `json:"type"`
	Page      domain.Page `json:"page"`
	Timestamp time.Time   `json:"timestamp"`
}

func Register(server *grpc.Server, service *app.Service, conn *jnats.Conn, subject string, logger *zap.Logger) {
	handler := &Server{service: service, conn: conn, subject: subject, logger: logger}
	pagesv1.RegisterPagesServer(server, handler)
	pagesv1.RegisterPagesRealtimeServer(server, handler)
}

func (server *Server) CreatePage(ctx context.Context, request *pagesv1.CreatePageRequest) (*pagesv1.CreatePageResponse, error) {
	blocks, err := blocksFromProto(request.GetBlocks())
	if err != nil {
		return nil, err
	}
	var cover *string
	if c := request.GetCover(); c != "" {
		cover = &c
	}
	// gRPC does not carry user context yet; owner_id left empty for internal use.
	page, err := server.service.CreatePage(ctx, "", request.GetTitle(), cover, blocks)
	if err != nil {
		return nil, mapError(err)
	}
	return &pagesv1.CreatePageResponse{Page: pageToProto(page)}, nil
}

func (server *Server) GetPage(ctx context.Context, request *pagesv1.GetPageRequest) (*pagesv1.GetPageResponse, error) {
	page, err := server.service.GetPage(ctx, domain.PageID(request.GetPageId()))
	if err != nil {
		return nil, mapError(err)
	}
	return &pagesv1.GetPageResponse{Page: pageToProto(page)}, nil
}

func (server *Server) UpdateBlocks(ctx context.Context, request *pagesv1.UpdateBlocksRequest) (*pagesv1.UpdateBlocksResponse, error) {
	blocks, err := blocksFromProto(request.GetBlocks())
	if err != nil {
		return nil, err
	}
	pageID := domain.PageID(request.GetPageId())
	// gRPC does not carry user context yet; ownership not checked for internal use.
	if err := server.service.UpdateBlocks(ctx, "", pageID, blocks); err != nil {
		return nil, mapError(err)
	}
	page, err := server.service.GetPage(ctx, pageID)
	if err != nil {
		return nil, mapError(err)
	}
	return &pagesv1.UpdateBlocksResponse{Page: pageToProto(page)}, nil
}

func (server *Server) SubscribePage(request *pagesv1.SubscribePageRequest, stream pagesv1.PagesRealtime_SubscribePageServer) error {
	subscription, err := server.conn.SubscribeSync(server.subject)
	if err != nil {
		return status.Errorf(codes.Unavailable, "subscribe nats: %v", err)
	}
	defer subscription.Unsubscribe()

	ctx := stream.Context()
	for {
		msg, err := subscription.NextMsgWithContext(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return status.Errorf(codes.Unavailable, "stream nats: %v", err)
		}

		var event pageEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			server.logger.Warn("invalid page event payload", zap.Error(err))
			continue
		}
		if request.GetPageId() != "" && string(event.Page.ID) != request.GetPageId() {
			continue
		}

		if err := stream.Send(&pagesv1.PageEvent{
			Type:      event.Type,
			Page:      pageToProto(event.Page),
			Timestamp: event.Timestamp.UTC().Format(time.RFC3339Nano),
		}); err != nil {
			return status.Errorf(codes.Unavailable, "send stream: %v", err)
		}
	}
}

func blocksFromProto(blocks []*pagesv1.Block) ([]domain.Block, error) {
	result := make([]domain.Block, 0, len(blocks))
	for _, block := range blocks {
		data := block.GetDataJson()
		if data == "" {
			data = "{}"
		}
		if !json.Valid([]byte(data)) {
			return nil, status.Error(codes.InvalidArgument, "block data_json must be valid json")
		}
		var parentID *string
		if block.GetParentId() != "" {
			value := block.GetParentId()
			parentID = &value
		}
		result = append(result, domain.Block{
			ID:       block.GetId(),
			PageID:   domain.PageID(block.GetPageId()),
			ParentID: parentID,
			Type:     domain.BlockType(block.GetType()),
			Position: int(block.GetPosition()),
			Data:     json.RawMessage(data),
		})
	}
	return result, nil
}

func pageToProto(page domain.Page) *pagesv1.Page {
	blocks := make([]*pagesv1.Block, 0, len(page.Blocks))
	for _, block := range page.Blocks {
		blocks = append(blocks, blockToProto(block))
	}

	pageProto := &pagesv1.Page{
		Id:        string(page.ID),
		Title:     page.Title,
		Blocks:    blocks,
		CreatedAt: page.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: page.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if page.DeletedAt != nil {
		pageProto.DeletedAt = page.DeletedAt.UTC().Format(time.RFC3339Nano)
	}
	return pageProto
}

func blockToProto(block domain.Block) *pagesv1.Block {
	data := string(block.Data)
	if data == "" {
		data = "{}"
	}
	protoBlock := &pagesv1.Block{
		Id:       block.ID,
		PageId:   string(block.PageID),
		Type:     string(block.Type),
		Position: int32(block.Position),
		DataJson: data,
	}
	if block.ParentID != nil {
		protoBlock.ParentId = *block.ParentID
	}
	return protoBlock
}

func mapError(err error) error {
	if errors.Is(err, errs.ErrInvalidInput) {
		return status.Error(codes.InvalidArgument, "invalid input")
	}
	if errors.Is(err, errs.ErrNotFound) {
		return status.Error(codes.NotFound, "not found")
	}
	return status.Error(codes.Internal, "internal error")
}
