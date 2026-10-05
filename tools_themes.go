package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ThemeFilesArgs struct {
	ThemeID   string   `json:"theme_id" jsonschema:"Theme id (required)"`
	Filenames []string `json:"filenames,omitempty" jsonschema:"Files to read, e.g. templates/index.json or sections/header.liquid. Wildcards work, e.g. templates/*.json. Omit to list all file names without contents."`
}

type ThemeFile struct {
	Filename string `json:"filename" jsonschema:"Path inside the theme, e.g. sections/header.liquid or assets/custom.css"`
	Content  string `json:"content" jsonschema:"Full text content of the file"`
}

type ThemeFileSaveArgs struct {
	ThemeID string      `json:"theme_id" jsonschema:"Theme id (required). Prefer an unpublished theme and publish it once checked."`
	Files   []ThemeFile `json:"files" jsonschema:"Files to create or overwrite (required)"`
}

type ThemeFileDeleteArgs struct {
	ThemeID   string   `json:"theme_id" jsonschema:"Theme id (required)"`
	Filenames []string `json:"filenames" jsonschema:"Files to delete (required)"`
}

type ThemeCreateArgs struct {
	SourceURL string `json:"source_url" jsonschema:"Public URL of a theme .zip (required)"`
	Name      string `json:"name,omitempty" jsonschema:"Optional theme name"`
}

const (
	themesDoc = `query { themes(first: 20) { nodes { id name role processing updatedAt } } }`

	themeFileListDoc = `query($id: ID!, $first: Int!) {
  theme(id: $id) { id name role files(first: $first) { nodes { filename size contentType updatedAt } pageInfo { hasNextPage endCursor } } }
}`

	themeFileReadDoc = `query($id: ID!, $filenames: [String!], $first: Int!) {
  theme(id: $id) {
    id name role
    files(filenames: $filenames, first: $first) {
      nodes { filename size body { ... on OnlineStoreThemeFileBodyText { content } ... on OnlineStoreThemeFileBodyUrl { url } } }
    }
  }
}`

	themeFileSaveDoc = `mutation($themeId: ID!, $files: [OnlineStoreThemeFilesUpsertFileInput!]!) {
  themeFilesUpsert(themeId: $themeId, files: $files) { upsertedThemeFiles { filename } userErrors { field message } }
}`

	themeFileDeleteDoc = `mutation($themeId: ID!, $files: [String!]!) {
  themeFilesDelete(themeId: $themeId, files: $files) { deletedThemeFiles { filename } userErrors { field message } }
}`

	themeCreateDoc = `mutation($source: URL!, $name: String) {
  themeCreate(source: $source, name: $name) { theme { id name role processing } userErrors { field message } }
}`

	themePublishDoc = `mutation($id: ID!) {
  themePublish(id: $id) { theme { id name role } userErrors { field message } }
}`
)

func registerThemes(s *mcp.Server) {
	registerDoc("theme_file_list", themeFileListDoc)
	registerDoc("theme_file_read", themeFileReadDoc)

	addGQL(s, "shopify_themes",
		"List themes with their role (MAIN is the live storefront). Needs the read_themes scope.",
		themesDoc, noVars)

	addCustom(s, "shopify_theme_files",
		"List a theme's files, or read specific files' contents (Liquid, JSON templates, CSS). Needs the read_themes scope.",
		func(ctx context.Context, c *client, a ThemeFilesArgs) (string, error) {
			id, err := gid("OnlineStoreTheme", a.ThemeID)
			if err != nil {
				return "", err
			}
			if len(a.Filenames) == 0 {
				d, err := c.call(ctx, "theme_file_list", themeFileListDoc, map[string]any{"id": id, "first": 250})
				if err != nil {
					return "", err
				}
				return pretty(d), nil
			}
			first := len(a.Filenames) * 10
			if first > 50 {
				first = 50
			}
			d, err := c.call(ctx, "theme_file_read", themeFileReadDoc, map[string]any{"id": id, "filenames": a.Filenames, "first": first})
			if err != nil {
				return "", err
			}
			return pretty(d), nil
		})

	addGQL(s, "shopify_theme_file_save",
		"Create or overwrite theme text files (Liquid, JSON, CSS, JS). Writes to the theme immediately, so target an unpublished theme and publish after checking. Theme code scopes need Shopify's approval for some apps.",
		themeFileSaveDoc, func(a ThemeFileSaveArgs) (map[string]any, error) {
			id, err := gid("OnlineStoreTheme", a.ThemeID)
			if err != nil {
				return nil, err
			}
			if len(a.Files) == 0 {
				return nil, fmt.Errorf("files is required")
			}
			files := make([]map[string]any, 0, len(a.Files))
			for _, f := range a.Files {
				if strings.TrimSpace(f.Filename) == "" {
					return nil, fmt.Errorf("every file needs a filename")
				}
				files = append(files, map[string]any{
					"filename": f.Filename,
					"body":     map[string]any{"type": "TEXT", "value": f.Content},
				})
			}
			return map[string]any{"themeId": id, "files": files}, nil
		})

	addGQL(s, "shopify_theme_file_delete",
		"Delete files from a theme.",
		themeFileDeleteDoc, func(a ThemeFileDeleteArgs) (map[string]any, error) {
			id, err := gid("OnlineStoreTheme", a.ThemeID)
			if err != nil {
				return nil, err
			}
			if len(a.Filenames) == 0 {
				return nil, fmt.Errorf("filenames is required")
			}
			return map[string]any{"themeId": id, "files": a.Filenames}, nil
		})

	addGQL(s, "shopify_theme_create",
		"Install a theme from a public .zip URL as an unpublished theme.",
		themeCreateDoc, func(a ThemeCreateArgs) (map[string]any, error) {
			if strings.TrimSpace(a.SourceURL) == "" {
				return nil, fmt.Errorf("source_url is required")
			}
			v := map[string]any{"source": a.SourceURL}
			setIf(v, "name", a.Name)
			return v, nil
		})

	addGQL(s, "shopify_theme_publish",
		"Make a theme the live storefront theme (role MAIN). This changes what shoppers see.",
		themePublishDoc, idVars("OnlineStoreTheme"))
}
