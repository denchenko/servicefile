package golang

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/denchenko/servicefile/pkg/servicefile"
)

type CommentParser struct {
	services      []service
	relationships []relationship
}

func NewCommentParser() *CommentParser {
	return &CommentParser{
		services:      make([]service, 0),
		relationships: make([]relationship, 0),
	}
}

func (cp *CommentParser) Parse(dir string, recursive bool) ([]*servicefile.ServiceFile, error) {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to walk the path: %w", err)
		}

		if info.IsDir() && !recursive && path != dir {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		if err := cp.parseFile(path); err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking the path: %w", err)
	}

	return cp.buildServiceFiles()
}

type service struct {
	name        string
	description string
	system      string
}

func (s service) String() string {
	return fmt.Sprintf("name: %s, description: %s, system: %s",
		s.name,
		s.description,
		s.system,
	)
}

type relationship struct {
	serviceName string
	action      string
	targetName  string
	technology  string
	description string
	proto       string
}

func (r relationship) String() string {
	return fmt.Sprintf("service_name: %s, action: %s, target_name: %s, technology: %s, proto: %s, description: %s",
		r.serviceName,
		r.action,
		r.targetName,
		r.technology,
		r.proto,
		r.description,
	)
}

func (cp *CommentParser) parseFile(path string) error {
	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}

	for _, cg := range f.Comments {
		var commentText strings.Builder
		for _, c := range cg.List {
			commentText.WriteString(c.Text)
			commentText.WriteString("\n")
		}
		cp.parseCommentGroup(commentText.String())
	}

	ast.Inspect(f, func(n ast.Node) bool {
		x, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		if x.Doc == nil {
			return true
		}

		var commentText strings.Builder
		for _, c := range x.Doc.List {
			commentText.WriteString(c.Text)
			commentText.WriteString("\n")
		}
		cp.parseCommentGroup(commentText.String())

		return true
	})

	return nil
}

func (cp *CommentParser) parseCommentGroup(commentGroup string) {
	if !strings.Contains(commentGroup, "service:") {
		return
	}

	lines := strings.Split(commentGroup, "\n")

	switch {
	case strings.Contains(commentGroup, "service:name"):
		cp.parseServiceDefinition(lines)
	default:
		cp.parseRelationshipDefinition(lines)
	}
}

func (cp *CommentParser) parseServiceDefinition(lines []string) {
	var s service

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		comment := cp.extractCommentText(line)
		if comment == "" {
			continue
		}

		if strings.HasPrefix(comment, "service:name") {
			parts := strings.SplitN(comment, " ", 2)
			if len(parts) == 2 {
				s.name = strings.TrimSpace(parts[1])
			}
			continue
		}

		if strings.HasPrefix(comment, "description:") {
			parts := strings.SplitN(comment, ":", 2)
			if len(parts) == 2 {
				s.description = strings.TrimSpace(parts[1])
			}
			continue
		}

		if strings.HasPrefix(comment, "system:") {
			parts := strings.SplitN(comment, ":", 2)
			if len(parts) == 2 {
				s.system = strings.TrimSpace(parts[1])
			}
			continue
		}
	}

	if s.name != "" {
		cp.services = append(cp.services, s)
	}
}

func (cp *CommentParser) parseRelationshipDefinition(lines []string) {
	var r relationship

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		comment := cp.extractCommentText(line)
		if comment == "" {
			continue
		}

		switch {
		case strings.HasPrefix(comment, "service:"):
			r.serviceName, r.action, r.targetName = cp.extractRelationshipInfo(comment)
			continue
		case strings.HasPrefix(comment, "technology:"):
			parts := strings.SplitN(comment, ":", 2)
			if len(parts) == 2 {
				r.technology = strings.TrimSpace(parts[1])
			}
			continue
		case strings.HasPrefix(comment, "description:"):
			parts := strings.SplitN(comment, ":", 2)
			if len(parts) == 2 {
				r.description = strings.TrimSpace(parts[1])
			}
			continue
		case strings.HasPrefix(comment, "proto:"):
			parts := strings.SplitN(comment, ":", 2)
			if len(parts) == 2 {
				r.proto = strings.TrimSpace(parts[1])
			}
			continue
		}
	}

	if r.action != "" {
		cp.relationships = append(cp.relationships, r)
	}
}

func (cp *CommentParser) extractCommentText(line string) string {
	comment := strings.TrimSpace(line)
	comment = strings.TrimPrefix(comment, "//")
	comment = strings.TrimPrefix(comment, "/*")
	comment = strings.TrimSuffix(comment, "*/")
	return strings.TrimSpace(comment)
}

// extractRelationshipInfo extracts the service name, action, and target name from a comment.
// Format: service:{service_name}:{action} [target_service] or service:{action} [target_service]
// Example: service:database:uses PostgreSQL
// Example: service:uses PostgreSQL
func (cp *CommentParser) extractRelationshipInfo(comment string) (serviceName, action, targetName string) {
	parts := strings.SplitN(comment, " ", 2)
	serviceActionPart := parts[0]

	serviceActionParts := strings.Split(serviceActionPart, ":")
	if len(serviceActionParts) >= 3 {
		// Format: service:{service_name}:{action}
		serviceName = serviceActionParts[1]
		action = serviceActionParts[2]
	} else if len(serviceActionParts) == 2 {
		// Format: service:{action}
		action = serviceActionParts[1]
	}

	// Extract target name if present
	if len(parts) > 1 {
		targetName = strings.TrimSpace(parts[1])
	}

	return serviceName, action, targetName
}

func (cp *CommentParser) buildServiceFiles() ([]*servicefile.ServiceFile, error) {
	if err := cp.validateNoMixedUsage(); err != nil {
		return nil, err
	}

	serviceFiles := make(map[string]*servicefile.ServiceFile)

	for _, s := range cp.services {
		serviceFiles[s.name] = &servicefile.ServiceFile{
			Version: servicefile.Version,
			Info: servicefile.Info{
				Name:        s.name,
				Description: s.description,
				System:      s.system,
			},
			Relationships: []servicefile.Relationship{},
		}
	}

	for _, r := range cp.relationships {
		serviceName, err := cp.determineServiceName(r, serviceFiles)
		if err != nil {
			return nil, fmt.Errorf("failed to determine service name: %w", err)
		}

		if _, exists := serviceFiles[serviceName]; !exists {
			serviceFiles[serviceName] = &servicefile.ServiceFile{
				Version: servicefile.Version,
				Info: servicefile.Info{
					Name: serviceName,
				},
				Relationships: []servicefile.Relationship{},
			}
		}

		relationship := servicefile.Relationship{
			Action: servicefile.RelationshipAction(r.action),
			Name:   r.targetName,
		}

		if r.technology != "" {
			relationship.Technology = r.technology
		}

		if r.description != "" {
			relationship.Description = r.description
		}

		if r.proto != "" {
			relationship.Proto = r.proto
		}

		serviceFiles[serviceName].Relationships = append(serviceFiles[serviceName].Relationships, relationship)
	}

	if len(serviceFiles) == 0 {
		return nil, fmt.Errorf("no services found")
	}

	result := make([]*servicefile.ServiceFile, 0, len(serviceFiles))
	for _, sf := range serviceFiles {
		sf.Sort()
		result = append(result, sf)
	}

	return result, nil
}

func (cp *CommentParser) validateNoMixedUsage() error {
	var (
		hasExplicit bool
		hasImplicit bool
	)

	for _, r := range cp.relationships {
		if r.serviceName != "" {
			hasExplicit = true
		} else {
			hasImplicit = true
		}
	}

	if hasExplicit && hasImplicit {
		return fmt.Errorf("mixed relationship definition patterns detected: some relationships use explicit patterns (service:name:action) while others use implicit patterns (service:action)")
	}

	return nil
}

func (cp *CommentParser) determineServiceName(r relationship, serviceFiles map[string]*servicefile.ServiceFile) (string, error) {
	if r.serviceName != "" {
		return r.serviceName, nil
	}

	for name := range serviceFiles {
		return name, nil
	}

	return "", fmt.Errorf("no service name found for relationship: %s", r)
}
