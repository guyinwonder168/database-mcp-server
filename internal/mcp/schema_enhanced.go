package mcp

import (
	"context"
	"sort"
	"sync"
	"time"
)

const defaultProfilingWorkers = 4

type tableProfilingTask struct {
	tableName  string
	tableInfo  TableInfo
	sampleRows []map[string]interface{}
}

type tableProfilingResult struct {
	profile TableProfile
}

// enhanceSchemaAnalysis performs advanced profiling on schema sample data.
func enhanceSchemaAnalysis(
	ctx context.Context,
	tableSchemas map[string]TableInfo,
	sampleData map[string][]map[string]interface{},
	maxWorkers int,
) *EnhancedSchemaAnalysis {
	if maxWorkers <= 0 {
		maxWorkers = defaultProfilingWorkers
	}

	tableProfiles := profileTablesConcurrently(ctx, tableSchemas, sampleData, maxWorkers)
	return &EnhancedSchemaAnalysis{
		Enabled:       true,
		MaxWorkers:    maxWorkers,
		ProfiledAt:    time.Now(),
		TableProfiles: tableProfiles,
	}
}

// profileTablesConcurrently profiles multiple tables using a bounded worker pool.
func profileTablesConcurrently(
	ctx context.Context,
	tableSchemas map[string]TableInfo,
	sampleData map[string][]map[string]interface{},
	maxWorkers int,
) []TableProfile {
	if maxWorkers <= 0 {
		maxWorkers = 1
	}

	tasks := buildProfilingTasks(tableSchemas, sampleData)
	if len(tasks) == 0 {
		return nil
	}

	results := make(chan tableProfilingResult, len(tasks))
	taskChan := make(chan tableProfilingTask)

	var workerWG sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for task := range taskChan {
				select {
				case <-ctx.Done():
					return
				default:
				}
				results <- tableProfilingResult{profile: profileSingleTable(task)}
			}
		}()
	}

	go func() {
		defer close(taskChan)
		for _, task := range tasks {
			select {
			case <-ctx.Done():
				return
			case taskChan <- task:
			}
		}
	}()

	go func() {
		workerWG.Wait()
		close(results)
	}()

	tableProfiles := make([]TableProfile, 0, len(tasks))
	for result := range results {
		tableProfiles = append(tableProfiles, result.profile)
	}
	sort.Slice(tableProfiles, func(i, j int) bool {
		return tableProfiles[i].TableName < tableProfiles[j].TableName
	})
	return tableProfiles
}

// mergeWithExistingSchema merges enhanced profiling into the existing response payload.
func mergeWithExistingSchema(existing *AnalyzeSchemaResult, enhanced *EnhancedSchemaAnalysis) {
	if existing == nil || enhanced == nil {
		return
	}
	existing.ColumnProfiling = enhanced
}

func buildProfilingTasks(
	tableSchemas map[string]TableInfo,
	sampleData map[string][]map[string]interface{},
) []tableProfilingTask {
	tableNames := make([]string, 0, len(tableSchemas))
	for tableName := range tableSchemas {
		tableNames = append(tableNames, tableName)
	}
	sort.Strings(tableNames)

	tasks := make([]tableProfilingTask, 0, len(tableNames))
	for _, tableName := range tableNames {
		tasks = append(tasks, tableProfilingTask{
			tableName:  tableName,
			tableInfo:  tableSchemas[tableName],
			sampleRows: sampleData[tableName],
		})
	}
	return tasks
}

func profileSingleTable(task tableProfilingTask) TableProfile {
	columns := make([]ColumnProfile, 0, len(task.tableInfo.Columns))
	for _, column := range task.tableInfo.Columns {
		profile, err := ProfileColumn(column, task.sampleRows)
		if err != nil || profile == nil {
			continue
		}
		columns = append(columns, *profile)
	}
	sort.Slice(columns, func(i, j int) bool {
		return columns[i].ColumnName < columns[j].ColumnName
	})

	return TableProfile{
		TableName:    task.tableName,
		Columns:      columns,
		SampleRowCnt: len(task.sampleRows),
	}
}
