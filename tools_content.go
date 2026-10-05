package main

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type PageSaveArgs struct {
	ID          string `json:"id,omitempty" jsonschema:"Page id to update. Omit to create."`
	Title       string `json:"title,omitempty" jsonschema:"Page title (required when creating)"`
	BodyHTML    string `json:"body_html,omitempty" jsonschema:"HTML body"`
	Handle      string `json:"handle,omitempty" jsonschema:"URL handle, e.g. about-us"`
	IsPublished *bool  `json:"is_published,omitempty" jsonschema:"Visible on the storefront (default for new pages: true)"`
}

type BlogCreateArgs struct {
	Title  string `json:"title" jsonschema:"Blog title, e.g. News (required)"`
	Handle string `json:"handle,omitempty" jsonschema:"URL handle"`
}

type ArticleSaveArgs struct {
	ID          string   `json:"id,omitempty" jsonschema:"Article id to update. Omit to create."`
	BlogID      string   `json:"blog_id,omitempty" jsonschema:"Blog id (required when creating)"`
	Title       string   `json:"title,omitempty" jsonschema:"Article title (required when creating)"`
	BodyHTML    string   `json:"body_html,omitempty" jsonschema:"HTML body"`
	SummaryHTML string   `json:"summary_html,omitempty" jsonschema:"HTML excerpt"`
	AuthorName  string   `json:"author_name,omitempty" jsonschema:"Author display name (required when creating)"`
	Handle      string   `json:"handle,omitempty" jsonschema:"URL handle"`
	Tags        []string `json:"tags,omitempty" jsonschema:"Tags"`
	ImageURL    string   `json:"image_url,omitempty" jsonschema:"Featured image URL"`
	ImageAlt    string   `json:"image_alt,omitempty" jsonschema:"Featured image alt text"`
	IsPublished *bool    `json:"is_published,omitempty" jsonschema:"Visible on the storefront (default for new articles: true)"`
}

type MenuLeaf struct {
	Title      string `json:"title" jsonschema:"Link text"`
	Type       string `json:"type" jsonschema:"FRONTPAGE, COLLECTION, COLLECTIONS, PRODUCT, CATALOG, PAGE, BLOG, ARTICLE, SEARCH, SHOP_POLICY or HTTP"`
	ResourceID string `json:"resource_id,omitempty" jsonschema:"Id of the collection, product, page, blog or article the link points to"`
	URL        string `json:"url,omitempty" jsonschema:"URL for type HTTP, or a path like /pages/about"`
}

type MenuItem struct {
	Title      string     `json:"title" jsonschema:"Link text"`
	Type       string     `json:"type" jsonschema:"FRONTPAGE, COLLECTION, COLLECTIONS, PRODUCT, CATALOG, PAGE, BLOG, ARTICLE, SEARCH, SHOP_POLICY or HTTP"`
	ResourceID string     `json:"resource_id,omitempty" jsonschema:"Id of the collection, product, page, blog or article the link points to"`
	URL        string     `json:"url,omitempty" jsonschema:"URL for type HTTP, or a path like /pages/about"`
	Children   []MenuLeaf `json:"children,omitempty" jsonschema:"Dropdown links under this item"`
}

type MenuSaveArgs struct {
	ID     string     `json:"id,omitempty" jsonschema:"Menu id to replace. Omit to create."`
	Title  string     `json:"title" jsonschema:"Menu title (required)"`
	Handle string     `json:"handle,omitempty" jsonschema:"Menu handle, e.g. main-menu or footer (required when creating)"`
	Items  []MenuItem `json:"items" jsonschema:"The complete list of top-level items. Replaces all existing items."`
}

type RedirectCreateArgs struct {
	Path   string `json:"path" jsonschema:"Old path, e.g. /old-page (required)"`
	Target string `json:"target" jsonschema:"New path or full URL (required)"`
}

type FileDef struct {
	URL      string `json:"url" jsonschema:"Public URL Shopify will download"`
	Alt      string `json:"alt,omitempty" jsonschema:"Alt text"`
	Filename string `json:"filename,omitempty" jsonschema:"Optional file name"`
}

type FileCreateArgs struct {
	Files []FileDef `json:"files" jsonschema:"Files to upload from public URLs (required)"`
}

type Metafield struct {
	OwnerID   string `json:"owner_id" jsonschema:"Full gid of the owner, e.g. gid://shopify/Product/123 or gid://shopify/Shop/1"`
	Namespace string `json:"namespace" jsonschema:"Namespace, e.g. custom"`
	Key       string `json:"key" jsonschema:"Key"`
	Type      string `json:"type" jsonschema:"Metafield type, e.g. single_line_text_field, multi_line_text_field, number_integer, boolean, json, url"`
	Value     string `json:"value" jsonschema:"Value as a string"`
}

type MetafieldsSetArgs struct {
	Metafields []Metafield `json:"metafields" jsonschema:"Up to 25 metafields to set"`
}

const (
	pagesDoc = `query($first: Int!, $after: String, $query: String) {
  pages(first: $first, after: $after, query: $query) {
    nodes { id title handle isPublished updatedAt }
    pageInfo { hasNextPage endCursor }
  }
}`

	pageCreateDoc = `mutation($page: PageCreateInput!) {
  pageCreate(page: $page) { page { id title handle isPublished } userErrors { field message } }
}`

	pageUpdateDoc = `mutation($id: ID!, $page: PageUpdateInput!) {
  pageUpdate(id: $id, page: $page) { page { id title handle isPublished } userErrors { field message } }
}`

	pageDeleteDoc = `mutation($id: ID!) {
  pageDelete(id: $id) { deletedPageId userErrors { field message } }
}`

	blogsDoc = `query { blogs(first: 20) { nodes { id title handle } } }`

	blogCreateDoc = `mutation($blog: BlogCreateInput!) {
  blogCreate(blog: $blog) { blog { id title handle } userErrors { field message } }
}`

	articlesDoc = `query($first: Int!, $after: String, $query: String) {
  articles(first: $first, after: $after, query: $query) {
    nodes { id title handle isPublished publishedAt tags blog { id title } }
    pageInfo { hasNextPage endCursor }
  }
}`

	articleCreateDoc = `mutation($article: ArticleCreateInput!) {
  articleCreate(article: $article) { article { id title handle isPublished blog { id title } } userErrors { field message } }
}`

	articleUpdateDoc = `mutation($id: ID!, $article: ArticleUpdateInput!) {
  articleUpdate(id: $id, article: $article) { article { id title handle isPublished } userErrors { field message } }
}`

	articleDeleteDoc = `mutation($id: ID!) {
  articleDelete(id: $id) { deletedArticleId userErrors { field message } }
}`

	menusDoc = `query {
  menus(first: 20) {
    nodes { id title handle items { id title type url resourceId items { id title type url resourceId } } }
  }
}`

	menuCreateDoc = `mutation($title: String!, $handle: String!, $items: [MenuItemCreateInput!]!) {
  menuCreate(title: $title, handle: $handle, items: $items) { menu { id title handle } userErrors { field message } }
}`

	menuUpdateDoc = `mutation($id: ID!, $title: String!, $handle: String, $items: [MenuItemUpdateInput!]!) {
  menuUpdate(id: $id, title: $title, handle: $handle, items: $items) { menu { id title handle } userErrors { field message } }
}`

	menuDeleteDoc = `mutation($id: ID!) {
  menuDelete(id: $id) { deletedMenuId userErrors { field message } }
}`

	redirectsDoc = `query($first: Int!, $after: String, $query: String) {
  urlRedirects(first: $first, after: $after, query: $query) {
    nodes { id path target }
    pageInfo { hasNextPage endCursor }
  }
}`

	redirectCreateDoc = `mutation($urlRedirect: UrlRedirectInput!) {
  urlRedirectCreate(urlRedirect: $urlRedirect) { urlRedirect { id path target } userErrors { field message } }
}`

	redirectDeleteDoc = `mutation($id: ID!) {
  urlRedirectDelete(id: $id) { deletedUrlRedirectId userErrors { field message } }
}`

	filesDoc = `query($first: Int!, $after: String, $query: String) {
  files(first: $first, after: $after, query: $query) {
    nodes {
      id alt fileStatus createdAt
      ... on MediaImage { image { url width height } }
      ... on GenericFile { url mimeType }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

	fileCreateDoc = `mutation($files: [FileCreateInput!]!) {
  fileCreate(files: $files) { files { id alt fileStatus } userErrors { field message } }
}`

	metafieldsSetDoc = `mutation($metafields: [MetafieldsSetInput!]!) {
  metafieldsSet(metafields: $metafields) { metafields { id namespace key value type } userErrors { field message } }
}`
)

var imageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".svg": true, ".avif": true}

func fileContentType(url string) string {
	p := url
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if imageExts[strings.ToLower(path.Ext(p))] {
		return "IMAGE"
	}
	return "FILE"
}

var menuTypes = []string{"FRONTPAGE", "COLLECTION", "COLLECTIONS", "PRODUCT", "CATALOG", "PAGE", "BLOG", "ARTICLE", "SEARCH", "SHOP_POLICY", "HTTP"}

// menuResourceKind maps a menu item type to the resource gid kind.
var menuResourceKind = map[string]string{
	"COLLECTION": "Collection", "PRODUCT": "Product", "PAGE": "Page", "BLOG": "Blog", "ARTICLE": "Article",
}

func menuNode(title, typ, resourceID, url string) (map[string]any, error) {
	t, err := oneOf("type", typ, menuTypes...)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("every menu item needs a title")
	}
	n := map[string]any{"title": title, "type": t}
	if resourceID != "" {
		kind, ok := menuResourceKind[t]
		if !ok {
			return nil, fmt.Errorf("menu item %q: type %s does not take a resource_id", title, t)
		}
		id, err := gid(kind, resourceID)
		if err != nil {
			return nil, err
		}
		n["resourceId"] = id
	} else if _, needs := menuResourceKind[t]; needs {
		return nil, fmt.Errorf("menu item %q: type %s needs a resource_id", title, t)
	}
	if t == "HTTP" && url == "" {
		return nil, fmt.Errorf("menu item %q: type HTTP needs a url", title)
	}
	setIf(n, "url", url)
	return n, nil
}

func buildMenuItems(items []MenuItem) ([]map[string]any, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("items is required")
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		n, err := menuNode(it.Title, it.Type, it.ResourceID, it.URL)
		if err != nil {
			return nil, err
		}
		if len(it.Children) > 0 {
			kids := make([]map[string]any, 0, len(it.Children))
			for _, k := range it.Children {
				kn, err := menuNode(k.Title, k.Type, k.ResourceID, k.URL)
				if err != nil {
					return nil, err
				}
				kids = append(kids, kn)
			}
			n["items"] = kids
		}
		out = append(out, n)
	}
	return out, nil
}

func registerContent(s *mcp.Server) {
	addGQL(s, "shopify_pages", "List online store pages (About, Contact, FAQ).", pagesDoc, listOnly)
	registerDoc("page_create", pageCreateDoc)
	registerDoc("page_update", pageUpdateDoc)
	addCustom(s, "shopify_page_save",
		"Create a page, or update one when id is given. New pages are published by default.",
		func(ctx context.Context, c *client, a PageSaveArgs) (string, error) {
			in := map[string]any{}
			setIf(in, "title", a.Title)
			setIf(in, "body", a.BodyHTML)
			setIf(in, "handle", a.Handle)
			if a.IsPublished != nil {
				in["isPublished"] = *a.IsPublished
			}
			if strings.TrimSpace(a.ID) == "" {
				if strings.TrimSpace(a.Title) == "" {
					return "", fmt.Errorf("title is required when creating a page")
				}
				if a.IsPublished == nil {
					in["isPublished"] = true
				}
				d, err := c.call(ctx, "page_create", pageCreateDoc, map[string]any{"page": in})
				if err != nil {
					return "", err
				}
				return pretty(d), nil
			}
			id, err := gid("Page", a.ID)
			if err != nil {
				return "", err
			}
			if len(in) == 0 {
				return "", fmt.Errorf("nothing to update: pass at least one field")
			}
			d, err := c.call(ctx, "page_update", pageUpdateDoc, map[string]any{"id": id, "page": in})
			if err != nil {
				return "", err
			}
			return pretty(d), nil
		})
	addGQL(s, "shopify_page_delete", "Delete a page permanently.", pageDeleteDoc, idVars("Page"))

	addGQL(s, "shopify_blogs", "List blogs.", blogsDoc, noVars)
	addGQL(s, "shopify_blog_create", "Create a blog that holds articles.", blogCreateDoc, func(a BlogCreateArgs) (map[string]any, error) {
		if strings.TrimSpace(a.Title) == "" {
			return nil, fmt.Errorf("title is required")
		}
		b := map[string]any{"title": a.Title}
		setIf(b, "handle", a.Handle)
		return map[string]any{"blog": b}, nil
	})
	addGQL(s, "shopify_articles", "List blog articles.", articlesDoc, listOnly)
	registerDoc("article_create", articleCreateDoc)
	registerDoc("article_update", articleUpdateDoc)
	addCustom(s, "shopify_article_save",
		"Create a blog article, or update one when id is given. New articles are published by default.",
		func(ctx context.Context, c *client, a ArticleSaveArgs) (string, error) {
			in := map[string]any{}
			setIf(in, "title", a.Title)
			setIf(in, "body", a.BodyHTML)
			setIf(in, "summary", a.SummaryHTML)
			setIf(in, "handle", a.Handle)
			setIf(in, "tags", a.Tags)
			if a.AuthorName != "" {
				in["author"] = map[string]any{"name": a.AuthorName}
			}
			if a.ImageURL != "" {
				img := map[string]any{"url": a.ImageURL}
				setIf(img, "altText", a.ImageAlt)
				in["image"] = img
			}
			if a.IsPublished != nil {
				in["isPublished"] = *a.IsPublished
			}
			if strings.TrimSpace(a.ID) == "" {
				if a.Title == "" || a.AuthorName == "" || a.BlogID == "" {
					return "", fmt.Errorf("title, author_name and blog_id are required when creating an article")
				}
				blog, err := gid("Blog", a.BlogID)
				if err != nil {
					return "", err
				}
				in["blogId"] = blog
				if a.IsPublished == nil {
					in["isPublished"] = true
				}
				d, err := c.call(ctx, "article_create", articleCreateDoc, map[string]any{"article": in})
				if err != nil {
					return "", err
				}
				return pretty(d), nil
			}
			id, err := gid("Article", a.ID)
			if err != nil {
				return "", err
			}
			if len(in) == 0 {
				return "", fmt.Errorf("nothing to update: pass at least one field")
			}
			d, err := c.call(ctx, "article_update", articleUpdateDoc, map[string]any{"id": id, "article": in})
			if err != nil {
				return "", err
			}
			return pretty(d), nil
		})
	addGQL(s, "shopify_article_delete", "Delete an article permanently.", articleDeleteDoc, idVars("Article"))

	addGQL(s, "shopify_menus", "List navigation menus (main-menu, footer) with their items.", menusDoc, noVars)
	registerDoc("menu_create", menuCreateDoc)
	registerDoc("menu_update", menuUpdateDoc)
	addCustom(s, "shopify_menu_save",
		"Create a navigation menu, or replace one when id is given. items is the complete list of links: pass every link, not just changes. Default menus (main-menu, footer) already exist: update them by id.",
		func(ctx context.Context, c *client, a MenuSaveArgs) (string, error) {
			if strings.TrimSpace(a.Title) == "" {
				return "", fmt.Errorf("title is required")
			}
			items, err := buildMenuItems(a.Items)
			if err != nil {
				return "", err
			}
			if strings.TrimSpace(a.ID) == "" {
				if strings.TrimSpace(a.Handle) == "" {
					return "", fmt.Errorf("handle is required when creating a menu")
				}
				d, err := c.call(ctx, "menu_create", menuCreateDoc, map[string]any{"title": a.Title, "handle": a.Handle, "items": items})
				if err != nil {
					return "", err
				}
				return pretty(d), nil
			}
			id, err := gid("Menu", a.ID)
			if err != nil {
				return "", err
			}
			v := map[string]any{"id": id, "title": a.Title, "items": items}
			setIf(v, "handle", a.Handle)
			d, err := c.call(ctx, "menu_update", menuUpdateDoc, v)
			if err != nil {
				return "", err
			}
			return pretty(d), nil
		})
	addGQL(s, "shopify_menu_delete", "Delete a navigation menu.", menuDeleteDoc, idVars("Menu"))

	addGQL(s, "shopify_redirects", "List URL redirects.", redirectsDoc, listOnly)
	addGQL(s, "shopify_redirect_create", "Create a 301 redirect from an old path to a new path or URL. Use when changing handles or migrating from another platform.", redirectCreateDoc, func(a RedirectCreateArgs) (map[string]any, error) {
		if strings.TrimSpace(a.Path) == "" || strings.TrimSpace(a.Target) == "" {
			return nil, fmt.Errorf("path and target are required")
		}
		if !strings.HasPrefix(a.Path, "/") {
			return nil, fmt.Errorf("path must start with /")
		}
		return map[string]any{"urlRedirect": map[string]any{"path": a.Path, "target": a.Target}}, nil
	})
	addGQL(s, "shopify_redirect_delete", "Delete a URL redirect.", redirectDeleteDoc, idVars("UrlRedirect"))

	addGQL(s, "shopify_files", "List uploaded files and images in the store's Files library.", filesDoc, listOnly)
	addGQL(s, "shopify_file_create", "Upload images or files to the store's Files library from public URLs.", fileCreateDoc, func(a FileCreateArgs) (map[string]any, error) {
		if len(a.Files) == 0 {
			return nil, fmt.Errorf("files is required")
		}
		files := make([]map[string]any, 0, len(a.Files))
		for _, f := range a.Files {
			if strings.TrimSpace(f.URL) == "" {
				return nil, fmt.Errorf("every file needs a url")
			}
			m := map[string]any{"originalSource": f.URL, "contentType": fileContentType(f.URL)}
			setIf(m, "alt", f.Alt)
			setIf(m, "filename", f.Filename)
			files = append(files, m)
		}
		return map[string]any{"files": files}, nil
	})

	addGQL(s, "shopify_metafields_set",
		"Set metafields (custom data) on products, collections, pages, customers, orders or the shop. Up to 25 per call.",
		metafieldsSetDoc, func(a MetafieldsSetArgs) (map[string]any, error) {
			if len(a.Metafields) == 0 || len(a.Metafields) > 25 {
				return nil, fmt.Errorf("pass between 1 and 25 metafields")
			}
			out := make([]map[string]any, 0, len(a.Metafields))
			for _, m := range a.Metafields {
				if !strings.HasPrefix(m.OwnerID, "gid://") {
					return nil, fmt.Errorf("owner_id must be a full gid, got %q", m.OwnerID)
				}
				if m.Namespace == "" || m.Key == "" || m.Type == "" {
					return nil, fmt.Errorf("every metafield needs namespace, key and type")
				}
				out = append(out, map[string]any{"ownerId": m.OwnerID, "namespace": m.Namespace, "key": m.Key, "type": m.Type, "value": m.Value})
			}
			return map[string]any{"metafields": out}, nil
		})
}
