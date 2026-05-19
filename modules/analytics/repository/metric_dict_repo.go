package repository

import (
	"github.com/Caknoooo/go-gin-clean-starter/database/entities"
	"gorm.io/gorm"
)

type MetricDictRepo struct {
	db *gorm.DB
}

func NewMetricDictRepo(db *gorm.DB) *MetricDictRepo {
	return &MetricDictRepo{db: db}
}

func (r *MetricDictRepo) hasTable() bool {
	return r.db.Migrator().HasTable(&entities.MetaMetricDict{})
}

func (r *MetricDictRepo) FindAll() ([]entities.MetaMetricDict, error) {
	if !r.hasTable() {
		return []entities.MetaMetricDict{}, nil
	}
	var records []entities.MetaMetricDict
	err := r.db.Order("category, metric_key").Find(&records).Error
	return records, err
}

func (r *MetricDictRepo) FindByKeys(keys []string) (map[string]entities.MetaMetricDict, error) {
	if len(keys) == 0 || !r.hasTable() {
		return map[string]entities.MetaMetricDict{}, nil
	}
	var records []entities.MetaMetricDict
	if err := r.db.Where("metric_key IN ?", keys).Find(&records).Error; err != nil {
		return nil, err
	}
	out := make(map[string]entities.MetaMetricDict, len(records))
	for _, rec := range records {
		out[rec.MetricKey] = rec
	}
	return out, nil
}
