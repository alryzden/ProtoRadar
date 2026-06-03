package postgres

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type DescriptorMetadataRepository struct {
	db *DB
}

func NewDescriptorMetadataRepository(db *DB) *DescriptorMetadataRepository {
	return &DescriptorMetadataRepository{db: db}
}

func (repo *DescriptorMetadataRepository) Save(ctx context.Context, moduleVersionID domain.ModuleVersionID, metadata domain.DescriptorMetadata) error {
	return repo.db.WithinTransaction(ctx, func(txCtx context.Context) error {
		_, err := repo.db.executor(txCtx).Exec(txCtx, `DELETE FROM proto_files WHERE module_version_id = $1`, moduleVersionID.String())
		if err != nil {
			return mapError(err)
		}

		now := time.Now().UTC()
		for _, file := range metadata.Files {
			fileID, err := repo.createProtoFile(txCtx, moduleVersionID, file, now)
			if err != nil {
				return err
			}
			if err := repo.createProtoImports(txCtx, fileID, file.Imports, now); err != nil {
				return err
			}
			if err := repo.createProtoServices(txCtx, moduleVersionID, fileID, file.PackageName, file.Services, now); err != nil {
				return err
			}
			for _, message := range file.Messages {
				if err := repo.createProtoMessage(txCtx, moduleVersionID, fileID, "", file.PackageName, message, now); err != nil {
					return err
				}
			}
			if err := repo.createProtoEnums(txCtx, moduleVersionID, fileID, file.PackageName, file.Enums, now); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repo *DescriptorMetadataRepository) GetByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.DescriptorMetadata, error) {
	files, fileIDs, err := repo.loadProtoFiles(ctx, moduleVersionID)
	if err != nil {
		return domain.DescriptorMetadata{}, err
	}
	for i := range files {
		fileID := fileIDs[i]
		imports, err := repo.loadProtoImports(ctx, fileID)
		if err != nil {
			return domain.DescriptorMetadata{}, err
		}
		services, err := repo.loadProtoServices(ctx, moduleVersionID, fileID)
		if err != nil {
			return domain.DescriptorMetadata{}, err
		}
		messages, err := repo.loadProtoMessages(ctx, moduleVersionID, fileID)
		if err != nil {
			return domain.DescriptorMetadata{}, err
		}
		enums, err := repo.loadProtoEnums(ctx, moduleVersionID, fileID, messages)
		if err != nil {
			return domain.DescriptorMetadata{}, err
		}
		files[i].Imports = imports
		files[i].Services = services
		files[i].Messages = messages
		files[i].Enums = enums
	}
	return domain.DescriptorMetadata{Files: files}, nil
}

func (repo *DescriptorMetadataRepository) GetSummaryByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.DescriptorMetadataSummary, error) {
	var summary domain.DescriptorMetadataSummary
	err := repo.db.executor(ctx).QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM proto_files WHERE module_version_id = $1),
			(SELECT count(DISTINCT package_name) FROM proto_files WHERE module_version_id = $1 AND package_name <> ''),
			(SELECT count(*) FROM proto_imports pi JOIN proto_files pf ON pf.id = pi.proto_file_id WHERE pf.module_version_id = $1),
			(SELECT count(*) FROM proto_services WHERE module_version_id = $1),
			(SELECT count(*) FROM proto_methods pm JOIN proto_services ps ON ps.id = pm.service_id WHERE ps.module_version_id = $1),
			(SELECT count(*) FROM proto_messages WHERE module_version_id = $1),
			(SELECT count(*) FROM proto_fields pfield JOIN proto_messages pm ON pm.id = pfield.message_id WHERE pm.module_version_id = $1),
			(SELECT count(*) FROM proto_enums WHERE module_version_id = $1),
			(SELECT count(*) FROM proto_enum_values pev JOIN proto_enums pe ON pe.id = pev.enum_id WHERE pe.module_version_id = $1)
	`, moduleVersionID.String()).Scan(
		&summary.FileCount,
		&summary.PackageCount,
		&summary.ImportCount,
		&summary.ServiceCount,
		&summary.MethodCount,
		&summary.MessageCount,
		&summary.FieldCount,
		&summary.EnumCount,
		&summary.EnumValueCount,
	)
	if err != nil {
		return domain.DescriptorMetadataSummary{}, mapError(err)
	}
	return summary, nil
}

func (repo *DescriptorMetadataRepository) createProtoFile(ctx context.Context, moduleVersionID domain.ModuleVersionID, file domain.ProtoFile, now time.Time) (string, error) {
	var id string
	err := repo.db.executor(ctx).QueryRow(ctx, `
		INSERT INTO proto_files (id, module_version_id, path, package_name, syntax, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)
		RETURNING id
	`, moduleVersionID.String(), file.Path, file.PackageName, file.Syntax, now).Scan(&id)
	return id, mapError(err)
}

func (repo *DescriptorMetadataRepository) createProtoImports(ctx context.Context, fileID string, imports []domain.ProtoImport, now time.Time) error {
	for _, protoImport := range imports {
		_, err := repo.db.executor(ctx).Exec(ctx, `
			INSERT INTO proto_imports (id, proto_file_id, import_path, is_public, is_weak, created_at)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)
		`, fileID, protoImport.Path, protoImport.Public, protoImport.Weak, now)
		if err != nil {
			return mapError(err)
		}
	}
	return nil
}

func (repo *DescriptorMetadataRepository) createProtoServices(ctx context.Context, moduleVersionID domain.ModuleVersionID, fileID string, packageName string, services []domain.ProtoService, now time.Time) error {
	for _, service := range services {
		var serviceID string
		err := repo.db.executor(ctx).QueryRow(ctx, `
			INSERT INTO proto_services (id, module_version_id, proto_file_id, package_name, name, full_name, created_at)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)
			RETURNING id
		`, moduleVersionID.String(), fileID, packageName, service.Name, service.FullName, now).Scan(&serviceID)
		if err != nil {
			return mapError(err)
		}
		for _, method := range service.Methods {
			_, err := repo.db.executor(ctx).Exec(ctx, `
				INSERT INTO proto_methods (id, service_id, name, input_type, output_type, client_streaming, server_streaming, created_at)
				VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7)
			`, serviceID, method.Name, method.InputType, method.OutputType, method.ClientStreaming, method.ServerStreaming, now)
			if err != nil {
				return mapError(err)
			}
		}
	}
	return nil
}

func (repo *DescriptorMetadataRepository) createProtoMessage(ctx context.Context, moduleVersionID domain.ModuleVersionID, fileID string, parentID string, packageName string, message domain.ProtoMessage, now time.Time) error {
	var id string
	err := repo.db.executor(ctx).QueryRow(ctx, `
		INSERT INTO proto_messages (id, module_version_id, proto_file_id, parent_message_id, package_name, name, full_name, created_at)
		VALUES (gen_random_uuid(), $1, $2, NULLIF($3, '')::uuid, $4, $5, $6, $7)
		RETURNING id
	`, moduleVersionID.String(), fileID, parentID, packageName, message.Name, message.FullName, now).Scan(&id)
	if err != nil {
		return mapError(err)
	}
	for _, field := range message.Fields {
		_, err := repo.db.executor(ctx).Exec(ctx, `
			INSERT INTO proto_fields (id, message_id, name, number, type, type_name, label, json_name, oneof_name, is_repeated, is_map, created_at)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, id, field.Name, field.Number, field.Type, field.TypeName, field.Label, field.JSONName, field.OneofName, field.IsRepeated, field.IsMap, now)
		if err != nil {
			return mapError(err)
		}
	}
	for _, nested := range message.Messages {
		if err := repo.createProtoMessage(ctx, moduleVersionID, fileID, id, packageName, nested, now); err != nil {
			return err
		}
	}
	return repo.createProtoEnums(ctx, moduleVersionID, fileID, packageName, message.Enums, now)
}

func (repo *DescriptorMetadataRepository) createProtoEnums(ctx context.Context, moduleVersionID domain.ModuleVersionID, fileID string, packageName string, enums []domain.ProtoEnum, now time.Time) error {
	for _, enum := range enums {
		var enumID string
		err := repo.db.executor(ctx).QueryRow(ctx, `
			INSERT INTO proto_enums (id, module_version_id, proto_file_id, package_name, name, full_name, created_at)
			VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)
			RETURNING id
		`, moduleVersionID.String(), fileID, packageName, enum.Name, enum.FullName, now).Scan(&enumID)
		if err != nil {
			return mapError(err)
		}
		for _, value := range enum.Values {
			_, err := repo.db.executor(ctx).Exec(ctx, `
				INSERT INTO proto_enum_values (id, enum_id, name, number, created_at)
				VALUES (gen_random_uuid(), $1, $2, $3, $4)
			`, enumID, value.Name, value.Number, now)
			if err != nil {
				return mapError(err)
			}
		}
	}
	return nil
}

func (repo *DescriptorMetadataRepository) loadProtoFiles(ctx context.Context, moduleVersionID domain.ModuleVersionID) ([]domain.ProtoFile, []string, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT id, path, package_name, syntax
		FROM proto_files
		WHERE module_version_id = $1
		ORDER BY path ASC
	`, moduleVersionID.String())
	if err != nil {
		return nil, nil, mapError(err)
	}
	defer rows.Close()

	files := make([]domain.ProtoFile, 0)
	ids := make([]string, 0)
	for rows.Next() {
		var file domain.ProtoFile
		var id string
		if err := rows.Scan(&id, &file.Path, &file.PackageName, &file.Syntax); err != nil {
			return nil, nil, err
		}
		ids = append(ids, id)
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, mapError(err)
	}
	if len(files) == 0 {
		return nil, nil, domain.ErrNotFound
	}
	return files, ids, nil
}

func (repo *DescriptorMetadataRepository) loadProtoImports(ctx context.Context, fileID string) ([]domain.ProtoImport, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT import_path, is_public, is_weak
		FROM proto_imports
		WHERE proto_file_id = $1
		ORDER BY import_path ASC
	`, fileID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	imports := make([]domain.ProtoImport, 0)
	for rows.Next() {
		var protoImport domain.ProtoImport
		if err := rows.Scan(&protoImport.Path, &protoImport.Public, &protoImport.Weak); err != nil {
			return nil, err
		}
		imports = append(imports, protoImport)
	}
	return imports, mapError(rows.Err())
}

func (repo *DescriptorMetadataRepository) loadProtoServices(ctx context.Context, moduleVersionID domain.ModuleVersionID, fileID string) ([]domain.ProtoService, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT id, name, full_name
		FROM proto_services
		WHERE module_version_id = $1 AND proto_file_id = $2
		ORDER BY full_name ASC
	`, moduleVersionID.String(), fileID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	services := make([]domain.ProtoService, 0)
	serviceIDs := make([]string, 0)
	for rows.Next() {
		var service domain.ProtoService
		var id string
		if err := rows.Scan(&id, &service.Name, &service.FullName); err != nil {
			return nil, err
		}
		serviceIDs = append(serviceIDs, id)
		services = append(services, service)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	for i := range services {
		methods, err := repo.loadProtoMethods(ctx, serviceIDs[i])
		if err != nil {
			return nil, err
		}
		services[i].Methods = methods
	}
	return services, nil
}

func (repo *DescriptorMetadataRepository) loadProtoMethods(ctx context.Context, serviceID string) ([]domain.ProtoMethod, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT name, input_type, output_type, client_streaming, server_streaming
		FROM proto_methods
		WHERE service_id = $1
		ORDER BY name ASC
	`, serviceID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	methods := make([]domain.ProtoMethod, 0)
	for rows.Next() {
		var method domain.ProtoMethod
		if err := rows.Scan(&method.Name, &method.InputType, &method.OutputType, &method.ClientStreaming, &method.ServerStreaming); err != nil {
			return nil, err
		}
		methods = append(methods, method)
	}
	return methods, mapError(rows.Err())
}

type protoMessageRow struct {
	id       string
	parentID string
	message  domain.ProtoMessage
}

func (repo *DescriptorMetadataRepository) loadProtoMessages(ctx context.Context, moduleVersionID domain.ModuleVersionID, fileID string) ([]domain.ProtoMessage, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT id, COALESCE(parent_message_id::text, ''), name, full_name
		FROM proto_messages
		WHERE module_version_id = $1 AND proto_file_id = $2
		ORDER BY full_name ASC
	`, moduleVersionID.String(), fileID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	messageRows := make([]protoMessageRow, 0)
	for rows.Next() {
		var row protoMessageRow
		if err := rows.Scan(&row.id, &row.parentID, &row.message.Name, &row.message.FullName); err != nil {
			return nil, err
		}
		fields, err := repo.loadProtoFields(ctx, row.id)
		if err != nil {
			return nil, err
		}
		row.message.Fields = fields
		messageRows = append(messageRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return buildMessageTree(messageRows), nil
}

func (repo *DescriptorMetadataRepository) loadProtoFields(ctx context.Context, messageID string) ([]domain.ProtoField, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT name, number, type, type_name, label, json_name, oneof_name, is_repeated, is_map
		FROM proto_fields
		WHERE message_id = $1
		ORDER BY number ASC
	`, messageID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	fields := make([]domain.ProtoField, 0)
	for rows.Next() {
		var field domain.ProtoField
		if err := rows.Scan(&field.Name, &field.Number, &field.Type, &field.TypeName, &field.Label, &field.JSONName, &field.OneofName, &field.IsRepeated, &field.IsMap); err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, mapError(rows.Err())
}

func (repo *DescriptorMetadataRepository) loadProtoEnums(ctx context.Context, moduleVersionID domain.ModuleVersionID, fileID string, messages []domain.ProtoMessage) ([]domain.ProtoEnum, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT id, name, full_name
		FROM proto_enums
		WHERE module_version_id = $1 AND proto_file_id = $2
		ORDER BY full_name ASC
	`, moduleVersionID.String(), fileID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	fileEnums := make([]domain.ProtoEnum, 0)
	for rows.Next() {
		var enum domain.ProtoEnum
		var id string
		if err := rows.Scan(&id, &enum.Name, &enum.FullName); err != nil {
			return nil, err
		}
		values, err := repo.loadProtoEnumValues(ctx, id)
		if err != nil {
			return nil, err
		}
		enum.Values = values
		if !enumBelongsToMessage(messages, enum) {
			fileEnums = append(fileEnums, enum)
		}
	}
	return fileEnums, mapError(rows.Err())
}

func (repo *DescriptorMetadataRepository) loadProtoEnumValues(ctx context.Context, enumID string) ([]domain.ProtoEnumValue, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT name, number
		FROM proto_enum_values
		WHERE enum_id = $1
		ORDER BY number ASC
	`, enumID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	values := make([]domain.ProtoEnumValue, 0)
	for rows.Next() {
		var value domain.ProtoEnumValue
		if err := rows.Scan(&value.Name, &value.Number); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, mapError(rows.Err())
}

func buildMessageTree(rows []protoMessageRow) []domain.ProtoMessage {
	children := map[string][]protoMessageRow{}
	for _, row := range rows {
		children[row.parentID] = append(children[row.parentID], row)
	}
	return buildMessageChildren("", children)
}

func buildMessageChildren(parentID string, children map[string][]protoMessageRow) []domain.ProtoMessage {
	rows := children[parentID]
	sort.Slice(rows, func(i int, j int) bool {
		return rows[i].message.FullName < rows[j].message.FullName
	})
	messages := make([]domain.ProtoMessage, 0, len(rows))
	for _, row := range rows {
		message := row.message
		message.Messages = buildMessageChildren(row.id, children)
		messages = append(messages, message)
	}
	return messages
}

func enumBelongsToMessage(messages []domain.ProtoMessage, enum domain.ProtoEnum) bool {
	for i := range messages {
		if strings.HasPrefix(enum.FullName, messages[i].FullName+".") {
			if enumBelongsToMessage(messages[i].Messages, enum) {
				return true
			}
			messages[i].Enums = append(messages[i].Enums, enum)
			return true
		}
	}
	return false
}

var _ domain.DescriptorMetadataRepository = (*DescriptorMetadataRepository)(nil)
