package main

import (
	"maps"
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

// pad 在字符串右侧添加空格，直到达到指定长度
func pad(s string, length int) string {
	if len(s) >= length {
		return s
	}
	return s + strings.Repeat(" ", length-len(s))
}

func makeTags(gormTag, jsonTag string) string {
	return fmt.Sprintf("`gorm:\"%s\" json:\"%s\"`", gormTag, jsonTag)
}

// snakeCase 将 PascalCase 或 camelCase 转换为 snake_case
func snakeCase(s string) string {
	var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")
	snake := matchFirstCap.ReplaceAllString(s, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

// ColumnDefinition 列定义
type ColumnDefinition struct {
	Name      string
	Type      string
	GormTag   string
	JsonTag   string
	FullTag   string
	Comment   string
	IsPrimary bool
	IsUnique  bool
}

// TableDefinition 表定义
type TableDefinition struct {
	TableName  string
	StructName string
	ModuleName string
	Columns    []ColumnDefinition   // 列信息
	Indexes    map[string][]string  // 索引信息
	HasTimeField   bool 			// 标记这张表是否包含 time.Time 类型的字段,是否需要导入 "time" 包
}

// ModelTemplateData 专门为 model.tpl 准备数据
type ModelTemplateData struct {
	*TableDefinition
	MaxNameLen int
	MaxTypeLen int
	MaxTagLen  int 		// 新增：用于存储最长标签的长度
	MaxFieldDefLen int  // 新增：用于对齐注释
}

// 模板函数映射
// var templateFuncs = template.FuncMap{
// 	"getType":          getGoTypeFromSQL, // getGoTypeFromSQL 本身是 OK 的
// 	"getTypeByColName": getGoTypeByColumnName,
// 	"lower":            strings.ToLower,
// 	"last":             lastIndex,
// 	"pascal":           pascalCase,
// 	"lowerCamel":       lowerCamelCase,
// 	"pad":              pad, 
// 	"makeTags":         makeTags,
// }

var templateFuncs = template.FuncMap{
	"lower":      strings.ToLower,
	"last":       lastIndex,
	"pascal":     pascalCase, // 我们将使用 pascalCase 这个 key
	"pascalCase": pascalCase, // 同时注册 pascalCase 以防万一
	"lowerCamel": lowerCamelCase,
	"pad":        pad,
	"makeTags":   makeTags,
	"add":        func(a, b int) int { return a + b },
	"subtract":   func(a, b int) int { return a - b },
	"snake":	  snakeCase,
}

// [新增] 定义一个用于过滤索引的列名列表 (PascalCase 格式)
// 任何索引只要包含了这个列表中的任何一个列，就不会为它生成查询方法。
var ignoredIndexColumns = map[string]bool{
    "DeletedAt": true,
    // 如果将来还有其他不希望生成方法的列，比如 "CreatedAt", "UpdatedAt"，可以直接加在这里
    // "CreatedAt": true, 
}

func main() {
	// 解析命令行参数
	sqlFile := flag.String("sql", "", "SQL建表语句文件路径")
	module := flag.String("module", "", "模块名称（如：shortlink）")
	output := flag.String("output", ".", "输出目录")
	flag.Parse()

	if *sqlFile == "" || *module == "" {
		fmt.Println("Usage: generator -sql <sql-file> -module <module-name> [-output <output-dir>]")
		os.Exit(1)
	}

	// 读取SQL文件
	sqlContent, err := os.ReadFile(*sqlFile)
	if err != nil {
		fmt.Printf("Error reading SQL file: %v\n", err)
		os.Exit(1)
	}

	// 解析SQL建表语句
	tableDef, err := parseSQLTableDefinition(string(sqlContent), *module)
	if err != nil {
		fmt.Printf("Error parsing SQL: %v\n", err)
		os.Exit(1)
	}

	// 创建输出目录
	if err := os.MkdirAll(*output, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// 生成代码文件
	if err := generateCodeFiles(tableDef, *output); err != nil {
		fmt.Printf("Error generating code files: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully generated code for module: %s\n", *module)
}

// parseSQLTableDefinition 解析SQL建表语句
func parseSQLTableDefinition(sql, moduleName string) (*TableDefinition, error) {
	// 1. 提取表名 (使用这个修正后的正则表达式)
	tableNameRegex := regexp.MustCompile("(?i)CREATE TABLE(?: IF NOT EXISTS)?\\s*`?(\\w+)`?")
	matches := tableNameRegex.FindStringSubmatch(sql)
	if len(matches) < 2 {
		return nil, fmt.Errorf("could not find table name in SQL")
	}
	tableName := matches[1]

	tableDef := &TableDefinition{
		TableName:  tableName,
		ModuleName: moduleName,
		StructName: pascalCase(moduleName), // 使用 pascalCase
		Indexes:    make(map[string][]string),
	}

	// 2. 提取括号内的核心定义体
	start := strings.Index(sql, "(")
	end := strings.LastIndex(sql, ")")
	if start == -1 || end == -1 {
		return nil, fmt.Errorf("could not find column definition block '(...)'")
	}
	definitions := sql[start+1 : end]

	// 3. 按行分割，逐行解析
	lines := strings.Split(definitions, "\n")
	gormModelFields := map[string]bool{"id": true, "created_at": true, "updated_at": true, "deleted_at": true}

	// 更精确的列定义正则表达式: 匹配 `column_name` type(len) ...
	columnRegex := regexp.MustCompile("^\\s*[`\"]?(\\w+)[`\"]?\\s+([\\w()]+(?:\\s+unsigned)?)")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		upperLine := strings.ToUpper(line)

		// 使用更健壮的判断逻辑来跳过非字段定义行
		if line == "" ||
		   strings.HasPrefix(upperLine, "PRIMARY KEY") ||
		   strings.HasPrefix(upperLine, "UNIQUE KEY") ||
		   strings.HasPrefix(upperLine, "KEY") ||
		   strings.HasPrefix(upperLine, "INDEX") ||
		   strings.HasPrefix(upperLine, "CONSTRAINT") {
		   continue
	   }

		match := columnRegex.FindStringSubmatch(line)
		if len(match) < 3 {
			continue
		}

		columnName := match[1]
		sqlType := match[2]

		if gormModelFields[columnName] {
			continue
		}

		// 提取注释
		commentRegex := regexp.MustCompile(`COMMENT\s+['"]([^'"]+)['"]`)
		commentMatch := commentRegex.FindStringSubmatch(line)
		comment := ""
		if len(commentMatch) > 1 {
			comment = commentMatch[1]
		}

		column := ColumnDefinition{
			Name:    pascalCase(columnName),     // 使用 pascalCase
			Type:    getGoTypeFromSQL(sqlType),  // 获取了 Go 类型
			JsonTag: lowerCamelCase(columnName), // json tag 使用小驼峰
			Comment: comment,
		}

		// 检查生成的 Go 类型是否是 "time.Time"
		// 如果是，就将我们新加的标志位置为 true。
		if column.Type == "time.Time" {
			tableDef.HasTimeField = true
		}
		
		// 为当前行构建 GORM Tag
		column.GormTag = buildGormTagForLine(columnName, sqlType, line)

		// 对json tag和注释行的间隔进行格式化设置
		column.FullTag = fmt.Sprintf("`gorm:\"%s\" json:\"%s\"`", column.GormTag, column.JsonTag)

		tableDef.Columns = append(tableDef.Columns, column)
	}

	// 4. 提取索引信息 (逻辑不变)
	parseIndexes(sql, tableDef)

	return tableDef, nil
}

// buildGormTagForLine 是一个新的、更可靠的 GORM tag 构建函数
func buildGormTagForLine(columnName, sqlType, line string) string {
	var tags []string
	tags = append(tags, "column:"+columnName) // 明确指定列名

	// 类型映射
	if strings.Contains(sqlType, "varchar") {
		re := regexp.MustCompile(`\((\d+)\)`)
		m := re.FindStringSubmatch(sqlType)
		if len(m) > 1 {
			tags = append(tags, "type:varchar("+m[1]+")")
		}
	} else if strings.Contains(sqlType, "datetime") {
		tags = append(tags, "type:datetime")
	} // ...可以为其他类型添加更多映射

	// 非空判断
	if strings.Contains(strings.ToUpper(line), "NOT NULL") {
		tags = append(tags, "not null")
	}

	// 默认值
	// re := regexp.MustCompile(`DEFAULT\\s+('([^']*)'|(\\S+))`)
	re := regexp.MustCompile(`DEFAULT\s+(?:'([^']*)'|(\S+))`)
	m := re.FindStringSubmatch(line)
	if len(m) > 1 {
		// m[2] for string default, m[3] for others like CURRENT_TIMESTAMP
		val := m[2]
		if val == "" {
			val = m[3]
		}
		tags = append(tags, "default:"+val)
	}
    
    // 注意: unique 约束通常由 UNIQUE KEY 定义，GORM 会自动从数据库读取。
    // 如果需要显式在 tag 中加 unique，最好在 parseIndexes 中处理。
    // 当前的 buildGormTagForLine 只处理行内约束。

	return strings.Join(tags, ";")
}

// parseIndexes 解析索引信息
func parseIndexes(sql string, tableDef *TableDefinition) {
	// 修正正则表达式，允许索引名被反引号包围，使其更健壮。
	// 原来的 `\w+` 无法匹配 `uk_short_code` 这样的字符串。
	// 新的 `\`?\w+\`?` 表示可选的反引号 + 单词字符 + 可选的反引号。
    uniqueIndexRegex := regexp.MustCompile("(?i)UNIQUE (?:KEY|INDEX)\\s+`?(\\w+)`?\\s*\\(([^)]+)\\)")
    indexRegex := regexp.MustCompile("(?i)(?:KEY|INDEX)\\s+`?(\\w+)`?\\s*\\(([^)]+)\\)")
    // 1. 解析唯一索引
	// 使用 FindAllStringSubmatch 捕获所有匹配项。
    uniqueMatches := uniqueIndexRegex.FindAllStringSubmatch(sql, -1)
    for _, match := range uniqueMatches {
		// match[0] 是完整匹配的字符串, e.g., "UNIQUE KEY `uk_short_code` (`short_code`)"
		// match[1] 是第一个捕获组 (索引名), e.g., "uk_short_code"
		// match[2] 是第二个捕获组 (列列表), e.g., "`short_code`"
        if len(match) > 2 {
            columns := parseIndexColumns(match[2])
            // [修改] 在添加索引前，检查是否应该被忽略
            if shouldIgnoreIndex(columns) {
                fmt.Printf("Skipping ignored unique index with columns: %v\n", columns) // 增加日志，方便调试
                continue
            }
            if len(columns) > 0 {
				// 为生成的 map key 添加 "Unique" 前缀以作区分。
                indexName := "Unique" + strings.Join(columns, "And")
                tableDef.Indexes[indexName] = columns
            }
        }
    }
    // 2. 解析普通索引
	// 需要排除掉已经被唯一索引规则匹配过的行，避免重复处理。
	// 我们可以先通过正则替换掉 UNIQUE KEY/INDEX，再匹配普通 KEY/INDEX。
    nonUniqueSQL := uniqueIndexRegex.ReplaceAllString(sql, "")
    indexMatches := indexRegex.FindAllStringSubmatch(nonUniqueSQL, -1)
    for _, match := range indexMatches {
        if len(match) > 2 {
            columns := parseIndexColumns(match[2])
            // [修改] 在添加索引前，检查是否应该被忽略
            if shouldIgnoreIndex(columns) {
                fmt.Printf("Skipping ignored index with columns: %v\n", columns) // 增加日志，方便调试
                continue
            }
            if len(columns) > 0 {
				// 为生成的 map key 添加 "By" 前缀。
                indexName := "By" + strings.Join(columns, "And")
                tableDef.Indexes[indexName] = columns
            }
        }
    }
}

// [新增] 检查一个索引是否应该被忽略的辅助函数
func shouldIgnoreIndex(columns []string) bool {
    for _, col := range columns {
        if _, exists := ignoredIndexColumns[col]; exists {
            return true // 只要索引中有一个列在忽略列表里，就整个忽略掉
        }
    }
    return false
}

// parseIndexColumns 解析索引列
func parseIndexColumns(columnStr string) []string {
	columns := strings.Split(columnStr, ",")
	var result []string
	for _, col := range columns {
		col = strings.TrimSpace(col)
		col = strings.Trim(col, "`\"")
		if col != "" {
			result = append(result, pascalCase(col))
		}
	}
	return result
}

// generateCodeFiles 生成代码文件
func generateCodeFiles(tableDef *TableDefinition, outputDir string) error {
	templates := map[string]string{
		"model.tpl":      filepath.Join(outputDir, "internal", "model", tableDef.ModuleName+".go"),
		"dto.tpl":        filepath.Join(outputDir, "internal", "dto", tableDef.ModuleName+".go"), 
		"repository.tpl": filepath.Join(outputDir, "internal", "repository", tableDef.ModuleName+".go"),
		"service.tpl":    filepath.Join(outputDir, "internal", "service", tableDef.ModuleName+".go"),
		"handler.tpl":    filepath.Join(outputDir, "internal", "api", "v1", tableDef.ModuleName+".go"),
	}

	for _, filePath := range templates {
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// --- 重新设计对齐数据计算逻辑，为 model.tpl 和 dto.tpl 准备特殊数据 ---
	maxNameLen := 0
	maxTypeLen := 0
	maxTagLen  := 0
	maxFieldDefLen := 0
	// 先计算数据库列的长度
	for _, col := range tableDef.Columns {
		if len(col.Name) > maxNameLen { maxNameLen = len(col.Name) }
		if len(col.Type) > maxTypeLen { maxTypeLen = len(col.Type) }
		// // 计算当前列的完整标签字符串长度
		// tagString := fmt.Sprintf("`gorm:\"%s\" json:\"%s\"`", col.GormTag, col.JsonTag)
		// if len(tagString) > maxTagLen {
		// 	maxTagLen = len(tagString)
		// }
	}
	// 在 Response DTO 中有硬编码的字段，也要把它们的长度考虑进去！
	baseFields := []struct{ Name, Type string }{
		{"ID", "uint"},
		{"CreatedAt", "time.Time"},
		{"UpdatedAt", "time.Time"},
	}
	for _, f := range baseFields {
		if len(f.Name) > maxNameLen { maxNameLen = len(f.Name) }
		if len(f.Type) > maxTypeLen { maxTypeLen = len(f.Type) }
	}
	
	// 计算 model.tpl 用的 gorm tag 最大长度
	for _, col := range tableDef.Columns {
		tagString := fmt.Sprintf("`gorm:\"%s\" json:\"%s\"`", col.GormTag, col.JsonTag)
		if len(tagString) > maxTagLen {
			maxTagLen = len(tagString)
		}
	}
	// 计算 dto.tpl 用的注释对齐所需的最大整行长度
	// 格式为: "  FieldName  Type  `json:"jsonTag"`  // Comment"
	for _, col := range tableDef.Columns {
		// (字段名+空格) + (类型+空格) + (json tag)
		lineLen := (maxNameLen + 1) + (maxTypeLen + 1) + len(fmt.Sprintf("`json:\"%s\"`", col.JsonTag))
		if lineLen > maxFieldDefLen {
			maxFieldDefLen = lineLen
		}
	}

	// 添加一些额外的buffer，让格式更好看
	modelData := ModelTemplateData{
		TableDefinition: tableDef,
		MaxNameLen:      maxNameLen + 1, 		// 加1个空格的缓冲
		MaxTypeLen:      maxTypeLen + 1, 		// 加1个空格的缓冲
		MaxTagLen:       maxTagLen + 2,  		// 加2个空格的缓冲
		MaxFieldDefLen:  maxFieldDefLen + 2, 	// 加2个空格的缓冲给注释
	}
	// --- 结束新增逻辑 ---

	for tplFile, outputFile := range templates {
        var data interface{}
        // 判断是否是 model.tpl 或者是 dto.tpl，都使用带有对齐信息的 modelData
        if tplFile == "model.tpl" || tplFile == "dto.tpl" {
            data = modelData
        } else {
            data = tableDef
        }

		if err := generateFromTemplate(tplFile, outputFile, data); err != nil {
			return fmt.Errorf("error generating %s: %v", outputFile, err)
		}
	}
	return nil
}

// generateFromTemplate 从模板生成文件
func generateFromTemplate(templateFile, outputFile string, data interface{}) error {
    if _, err := os.Stat(outputFile); err == nil {
        // 要生成的文件存在，则提示，并直接跳过，开始生成下一个文件
        fmt.Printf("File %s already exists, skipping.\n", outputFile)
        return nil
    }
	
	// 动态创建并注册 getTypeByColName
	// 复制全局 funcMap，避免并发问题
	localFuncs := make(template.FuncMap)
	maps.Copy(localFuncs, templateFuncs) // 把templateFuncs中的字段复制到localFuncs中
	// 动态创建一个能访问当前 data 中 columns 的函数
	localFuncs["getTypeByColName"] = func(name string) string {
		var columns []ColumnDefinition
		// 类型断言，判断 data 是哪种类型，然后提取 Columns
		if td, ok := data.(*TableDefinition); ok {
			columns = td.Columns
		} else if mtd, ok := data.(ModelTemplateData); ok {
			columns = mtd.TableDefinition.Columns
		} else {
			return "interface{}" // 无法确定类型，返回一个安全的默认值
		}
		for _, col := range columns {
			// 注意：模板中传入的列名是 PascalCase 的
			if col.Name == name {
				return col.Type
			}
		}
		return "string" // 找不到时的备用类型
	}

	// 使用新的 localFuncs 来解析模板
	tmpl, err := template.New(filepath.Base(templateFile)).Funcs(localFuncs).ParseFiles("templates/" + templateFile)
	if err != nil {
		return err
	}

	file, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	if err := tmpl.Execute(writer, data); err != nil {
		return err
	}

	return writer.Flush()
}

// 辅助函数
// pascalCase 将 snake_case 转换为 PascalCase (大驼峰)
// 例如： "user_name" -> "UserName"
func pascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// lowerCamelCase 将 snake_case 转换为 lowerCamelCase (小驼峰)
// 例如： "user_name" -> "userName"
func lowerCamelCase(s string) string {
	s = pascalCase(s)
	if len(s) > 0 {
		return strings.ToLower(s[:1]) + s[1:]
	}
	return ""
}

func getGoTypeFromSQL(sqlType string) string {
	sqlType = strings.ToLower(sqlType)
	switch {
	case strings.Contains(sqlType, "bigint"):
		return "int64"
	case strings.Contains(sqlType, "int") && !strings.Contains(sqlType, "bigint"):
		return "int"
	case strings.Contains(sqlType, "tinyint"):
		return "int8"
	case strings.Contains(sqlType, "varchar"), strings.Contains(sqlType, "text"), strings.Contains(sqlType, "char"):
		return "string"
	case strings.Contains(sqlType, "datetime"), strings.Contains(sqlType, "timestamp"):
		return "time.Time"
	case strings.Contains(sqlType, "bool"):
		return "bool"
	case strings.Contains(sqlType, "float"), strings.Contains(sqlType, "double"), strings.Contains(sqlType, "decimal"):
		return "float64"
	default:
		return "string"
	}
}

func lastIndex(index int, array []string) bool {
	return index == len(array)-1
}
