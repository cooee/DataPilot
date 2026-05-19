package service

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/Caknoooo/go-gin-clean-starter/database/entities"
	"github.com/Caknoooo/go-gin-clean-starter/modules/analytics/catalog"
	"github.com/Caknoooo/go-gin-clean-starter/modules/analytics/dto"
	"github.com/Caknoooo/go-gin-clean-starter/modules/analytics/repository"
)

type AnalyticsService interface {
	Query(ctx context.Context, req *dto.QueryRequest) (*dto.QueryResult, *dto.QueryMeta, error)
	GetSchema(ctx context.Context) (*dto.SchemaResponse, error)
}

type analyticsService struct {
	chRepo     *repository.CHQueryRepo
	metricRepo *repository.MetricDictRepo
}

func NewAnalyticsService(chRepo *repository.CHQueryRepo, metricRepo *repository.MetricDictRepo) AnalyticsService {
	return &analyticsService{chRepo: chRepo, metricRepo: metricRepo}
}

func (s *analyticsService) Query(ctx context.Context, req *dto.QueryRequest) (*dto.QueryResult, *dto.QueryMeta, error) {
	built, colMeta, err := buildQuery(req)
	if err != nil {
		return nil, nil, err
	}

	rows, err := s.chRepo.Query(ctx, built.SQL, built.Args)
	if err != nil {
		return nil, nil, err
	}

	metricKeys := collectMetricKeys(colMeta)
	dict, err := s.metricRepo.FindByKeys(metricKeys)
	if err != nil {
		return nil, nil, fmt.Errorf("load metric dict: %w", err)
	}
	enrichColumnMeta(colMeta, dict)

	limit := req.Limit
	if limit <= 0 {
		limit = defaultQueryLimit
	}
	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}

	meta := &dto.QueryMeta{
		Dataset:   req.Dataset,
		RowCount:  len(rows),
		Truncated: len(rows) >= limit,
	}
	if os.Getenv("ANALYTICS_DEBUG_SQL") == "true" {
		meta.SQL = built.SQL
	}

	return &dto.QueryResult{Columns: colMeta, Rows: rows}, meta, nil
}

func (s *analyticsService) GetSchema(ctx context.Context) (*dto.SchemaResponse, error) {
	_ = ctx
	allMetrics, err := s.metricRepo.FindAll()
	if err != nil {
		return nil, err
	}
	dict := make(map[string]entities.MetaMetricDict, len(allMetrics))
	for _, m := range allMetrics {
		dict[m.MetricKey] = m
	}

	names := make([]string, 0, len(catalog.Datasets))
	for name := range catalog.Datasets {
		names = append(names, name)
	}
	sort.Strings(names)

	resp := &dto.SchemaResponse{Datasets: make([]dto.DatasetSchema, 0, len(names))}
	for _, name := range names {
		ds := catalog.Datasets[name]
		item := dto.DatasetSchema{
			Name:        ds.Name,
			Label:       ds.Label,
			Description: ds.Description,
			DateColumn:  ds.DateColumn,
		}

		dimNames := sortedKeys(ds.Dimensions)
		for _, k := range dimNames {
			item.Dimensions = append(item.Dimensions, dto.FieldSchema{
				Name: k,
				Kind: string(ds.Dimensions[k].Kind),
			})
		}

		metricNames := sortedMetricKeys(ds.Metrics)
		for _, k := range metricNames {
			def := ds.Metrics[k]
			mfs := dto.MetricFieldSchema{
				Name:      k,
				Kind:      string(def.Kind),
				AggFunc:   string(def.AggFunc),
				MetricKey: def.MetricKey,
			}
			if def.MetricKey != "" {
				if meta, ok := dict[def.MetricKey]; ok {
					mfs.MetricNameZH = meta.MetricNameZH
					mfs.Unit = meta.Unit
					mfs.Direction = meta.Direction
					mfs.Description = meta.Description
				}
			}
			item.Metrics = append(item.Metrics, mfs)
		}
		resp.Datasets = append(resp.Datasets, item)
	}
	return resp, nil
}

func collectMetricKeys(cols []dto.ColumnMeta) []string {
	seen := make(map[string]struct{})
	var keys []string
	for _, c := range cols {
		if c.MetricKey == "" {
			continue
		}
		if _, ok := seen[c.MetricKey]; ok {
			continue
		}
		seen[c.MetricKey] = struct{}{}
		keys = append(keys, c.MetricKey)
	}
	return keys
}

func enrichColumnMeta(cols []dto.ColumnMeta, dict map[string]entities.MetaMetricDict) {
	for i := range cols {
		if cols[i].MetricKey == "" {
			continue
		}
		if meta, ok := dict[cols[i].MetricKey]; ok {
			cols[i].MetricNameZH = meta.MetricNameZH
			cols[i].Unit = meta.Unit
			cols[i].Direction = meta.Direction
			cols[i].Description = meta.Description
		}
	}
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedMetricKeys(m map[string]catalog.MetricDef) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
