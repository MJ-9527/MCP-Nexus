package repository

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"MCP-Nexus/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresAuditLogRepository struct{ pool *pgxpool.Pool }

func NewPostgresAuditLogRepository(pool *pgxpool.Pool) *PostgresAuditLogRepository {
	return &PostgresAuditLogRepository{pool: pool}
}

// auditLogColumns 与 audit_logs 表列顺序保持一致，便于 BatchCreate 统一构造行。
var auditLogColumns = []string{
	"request_id", "user_id", "tool_id", "server_id", "tool_name",
	"caller_role", "duration_ms", "status", "http_status", "denied_reason",
	"reject_reason", "params_summary", "params_sensitive_masked", "params_digest", "cost_estimate",
}

func (r *PostgresAuditLogRepository) Create(ctx context.Context, log *model.AuditLog) error {
	if log == nil {
		return errors.New("nil audit log")
	}
	const query = `INSERT INTO audit_logs
		(request_id,user_id,tool_id,server_id,tool_name,caller_role,duration_ms,status,http_status,denied_reason,reject_reason,params_summary,params_sensitive_masked,params_digest,cost_estimate)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING id, created_at`
	return r.pool.QueryRow(ctx, query, log.RequestID, log.UserID, log.ToolID, log.ServerID, log.ToolName, log.CallerRole, log.DurationMS, log.Status, log.HTTPStatus, log.DeniedReason, log.RejectReason, log.ParamsSummary, log.ParamsSensitiveMasked, log.ParamsDigest, log.CostEstimate).Scan(&log.ID, &log.CreatedAt)
}

// BatchCreate 批量写入（B14）。
// 使用 multi-row INSERT 单次往返完成整批插入，显著降低逐条 INSERT 的 IO 与
// SQL 解析开销；created_at 走表 DEFAULT NOW()，无需调用方填充。
// 空切片直接返回；超大批次（>1000 行）建议调用方自行分批以控制 SQL 长度。
func (r *PostgresAuditLogRepository) BatchCreate(ctx context.Context, logs []*model.AuditLog) error {
	if len(logs) == 0 {
		return nil
	}
	// 过滤 nil 并构造参数与占位符
	const colsPerRow = 15
	args := make([]any, 0, len(logs)*colsPerRow)
	placeholders := make([]string, 0, len(logs))
	rowIdx := 0 // 实际追加的行号（用于占位符序号），跳过 nil 后保持连续
	for _, log := range logs {
		if log == nil {
			continue
		}
		base := rowIdx * colsPerRow
		// 占位符：$1..$15, $16..$30, ...
		phs := make([]string, colsPerRow)
		for j := 0; j < colsPerRow; j++ {
			phs[j] = "$" + strconv.Itoa(base+j+1)
		}
		placeholders = append(placeholders, "("+strings.Join(phs, ",")+")")
		args = append(args,
			log.RequestID, log.UserID, log.ToolID, log.ServerID, log.ToolName,
			log.CallerRole, log.DurationMS, log.Status, log.HTTPStatus, log.DeniedReason,
			log.RejectReason, log.ParamsSummary, log.ParamsSensitiveMasked, log.ParamsDigest, log.CostEstimate,
		)
		rowIdx++
	}
	if len(placeholders) == 0 {
		return nil
	}
	query := "INSERT INTO audit_logs (" + strings.Join(auditLogColumns, ",") + ") VALUES " + strings.Join(placeholders, ",")
	_, err := r.pool.Exec(ctx, query, args...)
	return err
}

func (r *PostgresAuditLogRepository) List(ctx context.Context, filter AuditLogFilter) ([]*model.AuditLog, int64, error) {
	args := make([]any, 0, 7)
	conditions := make([]string, 0, 7)
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, strings.Replace(condition, "?", "$"+strconv.Itoa(len(args)), 1))
	}
	if filter.RequestID != "" {
		add("request_id = ?", filter.RequestID)
	}
	if filter.UserID != nil {
		add("user_id = ?", *filter.UserID)
	}
	if filter.ToolID != nil {
		add("tool_id = ?", *filter.ToolID)
	}
	if filter.ServerID != nil {
		add("server_id = ?", *filter.ServerID)
	}
	if filter.Status != "" {
		add("status = ?", filter.Status)
	}
	if filter.StartTime != nil {
		add("created_at >= ?", *filter.StartTime)
	}
	if filter.EndTime != nil {
		add("created_at <= ?", *filter.EndTime)
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	var total int64
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM audit_logs"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	pageSize := filter.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize
	query := `SELECT id,request_id,user_id,tool_id,server_id,tool_name,caller_role,duration_ms,status,http_status,denied_reason,reject_reason,params_summary,params_sensitive_masked,params_digest,cost_estimate,created_at FROM audit_logs` + where +
		" ORDER BY created_at DESC, id DESC LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
	args = append(args, pageSize, offset)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	logs := make([]*model.AuditLog, 0)
	for rows.Next() {
		log := new(model.AuditLog)
		if err := rows.Scan(&log.ID, &log.RequestID, &log.UserID, &log.ToolID, &log.ServerID, &log.ToolName, &log.CallerRole, &log.DurationMS, &log.Status, &log.HTTPStatus, &log.DeniedReason, &log.RejectReason, &log.ParamsSummary, &log.ParamsSensitiveMasked, &log.ParamsDigest, &log.CostEstimate, &log.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}
