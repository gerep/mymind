import {
  Plugin,
  PluginSettingTab,
  App,
  Setting,
  Modal,
  Notice,
  requestUrl,
  normalizePath,
} from "obsidian";

// ─── Types ──────────────────────────────────────────────────

interface MymindSettings {
  apiKey: string;
  model: string;
  prompt: string;
  bookmarksFolder: string;
}

interface AIResult {
  title: string;
  summary: string;
}

interface ContentInput {
  kind: "link" | "image" | "pdf";
  source: string;
  text?: string;
  imageBase64?: string;
  imageMIME?: string;
  imageExt?: string;
}

// ─── Constants ──────────────────────────────────────────────

const DEFAULT_SETTINGS: MymindSettings = {
  apiKey: "",
  model: "gemini-2.0-flash",
  prompt: "",
  bookmarksFolder: "bookmarks",
};

const IMAGE_EXTENSIONS: Record<string, string> = {
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".png": "image/png",
  ".gif": "image/gif",
  ".webp": "image/webp",
  ".bmp": "image/bmp",
};

const MAX_EXTRACT_CHARS = 12000;

// ─── Plugin ─────────────────────────────────────────────────

export default class MymindPlugin extends Plugin {
  settings!: MymindSettings;

  async onload() {
    await this.loadSettings();

    this.addCommand({
      id: "bookmark-url",
      name: "Bookmark URL",
      callback: () => new BookmarkURLModal(this).open(),
    });

    this.addCommand({
      id: "bookmark-clipboard",
      name: "Bookmark clipboard image",
      callback: () => this.bookmarkClipboard(),
    });

    this.addCommand({
      id: "bookmark-file",
      name: "Bookmark file (image or PDF)",
      callback: () => this.bookmarkFile(),
    });

    this.addSettingTab(new MymindSettingTab(this.app, this));
  }

  async loadSettings() {
    this.settings = Object.assign({}, DEFAULT_SETTINGS, await this.loadData());
  }

  async saveSettings() {
    await this.saveData(this.settings);
  }

  async bookmarkURL(url: string) {
    if (!this.settings.apiKey) {
      new Notice("mymind: Set your Gemini API key in settings");
      return;
    }

    const notice = new Notice("mymind: Analyzing...", 0);
    try {
      const content = await resolveURL(url);
      const result = await analyzeWithGemini(this.settings, content);
      const path = await this.writeBookmark(content, result);
      notice.hide();
      new Notice(`mymind: Saved to ${path}`);
    } catch (e: any) {
      notice.hide();
      new Notice(`mymind: ${e.message}`);
    }
  }

  async bookmarkClipboard() {
    if (!this.settings.apiKey) {
      new Notice("mymind: Set your Gemini API key in settings");
      return;
    }

    const notice = new Notice("mymind: Analyzing clipboard image...", 0);
    try {
      const { base64, mime } = await readClipboardImage();
      const content: ContentInput = {
        kind: "image",
        source: "clipboard",
        imageBase64: base64,
        imageMIME: mime,
        imageExt: extFromMIME(mime),
      };
      const result = await analyzeWithGemini(this.settings, content);
      const path = await this.writeBookmark(content, result);
      notice.hide();
      new Notice(`mymind: Saved to ${path}`);
    } catch (e: any) {
      notice.hide();
      new Notice(`mymind: ${e.message}`);
    }
  }

  async bookmarkFile() {
    if (!this.settings.apiKey) {
      new Notice("mymind: Set your Gemini API key in settings");
      return;
    }

    const input = document.createElement("input");
    input.type = "file";
    input.accept = "image/*,application/pdf";
    input.addEventListener("change", async () => {
      const file = input.files?.[0];
      if (!file) return;

      const notice = new Notice(`mymind: Analyzing ${file.name}...`, 0);
      try {
        const buffer = await file.arrayBuffer();
        const base64 = arrayBufferToBase64(buffer);
        const isPdf =
          file.type === "application/pdf" ||
          file.name.toLowerCase().endsWith(".pdf");

        let content: ContentInput;
        if (isPdf) {
          content = {
            kind: "pdf",
            source: file.name,
            imageBase64: base64,
            imageMIME: "application/pdf",
          };
        } else {
          const mime = file.type || "image/png";
          content = {
            kind: "image",
            source: file.name,
            imageBase64: base64,
            imageMIME: mime,
            imageExt: extFromMIME(mime),
          };
        }

        const result = await analyzeWithGemini(this.settings, content);
        const path = await this.writeBookmark(content, result);
        notice.hide();
        new Notice(`mymind: Saved to ${path}`);
      } catch (e: any) {
        notice.hide();
        new Notice(`mymind: ${e.message}`);
      }
    });
    input.click();
  }

  async writeBookmark(
    content: ContentInput,
    result: AIResult
  ): Promise<string> {
    const folder = this.settings.bookmarksFolder;

    if (!this.app.vault.getAbstractFileByPath(folder)) {
      await this.app.vault.createFolder(folder);
    }

    const slug = slugify(result.title) || "bookmark";
    let mdPath = normalizePath(`${folder}/${slug}.md`);
    mdPath = this.uniquePath(mdPath);

    const baseName = mdPath
      .replace(/\.md$/, "")
      .split("/")
      .pop()!;

    let attachmentName = "";
    if (content.imageBase64) {
      if (content.kind === "image") {
        attachmentName = baseName + (content.imageExt || ".png");
      } else if (content.kind === "pdf") {
        attachmentName = baseName + ".pdf";
      }
      if (attachmentName) {
        const filePath = normalizePath(`${folder}/${attachmentName}`);
        const buffer = base64ToArrayBuffer(content.imageBase64);
        await this.app.vault.createBinary(filePath, buffer);
      }
    }

    const md = buildMarkdown(content, result, attachmentName);
    await this.app.vault.create(mdPath, md);

    return mdPath;
  }

  uniquePath(path: string): string {
    if (!this.app.vault.getAbstractFileByPath(path)) return path;
    const ext = path.match(/\.[^.]+$/)?.[0] || "";
    const base = path.slice(0, path.length - ext.length);
    for (let i = 2; ; i++) {
      const candidate = `${base}-${i}${ext}`;
      if (!this.app.vault.getAbstractFileByPath(candidate)) return candidate;
    }
  }
}

// ─── URL Resolution ─────────────────────────────────────────

async function resolveURL(url: string): Promise<ContentInput> {
  if (isTweetURL(url)) {
    try {
      const extracted = await extractTweet(url);
      return {
        kind: "link",
        source: url,
        text: `Title: ${extracted.title}\n\n${extracted.text}`,
      };
    } catch {
      // Fall through to regular extraction
    }
  }

  if (isImageURL(url)) {
    const img = await downloadImage(url);
    return {
      kind: "image",
      source: url,
      imageBase64: img.base64,
      imageMIME: img.mime,
      imageExt: img.ext,
    };
  }

  try {
    const extracted = await extractFromURL(url);
    return {
      kind: "link",
      source: url,
      text: `Title: ${extracted.title}\n\n${extracted.text}`,
    };
  } catch {
    return { kind: "link", source: url, text: url };
  }
}

// ─── Extraction ─────────────────────────────────────────────

function isTweetURL(url: string): boolean {
  try {
    const u = new URL(url);
    const host = u.hostname.toLowerCase();
    return (
      (host.includes("twitter.com") || host.includes("x.com")) &&
      u.pathname.includes("/status/")
    );
  } catch {
    return false;
  }
}

async function extractTweet(
  url: string
): Promise<{ title: string; text: string }> {
  const endpoint = `https://publish.twitter.com/oembed?url=${encodeURIComponent(url)}&omit_script=1&dnt=1`;
  const resp = await requestUrl({ url: endpoint });
  const data = resp.json;

  const parser = new DOMParser();
  const doc = parser.parseFromString(data.html, "text/html");
  const text = collapseWhitespace(doc.body?.textContent?.trim() || "");

  return { title: `Tweet by ${data.author_name}`, text };
}

function isImageURL(url: string): boolean {
  try {
    const u = new URL(url);
    const ext = "." + u.pathname.split(".").pop()?.toLowerCase();
    return ext in IMAGE_EXTENSIONS;
  } catch {
    return false;
  }
}

async function downloadImage(
  url: string
): Promise<{ base64: string; mime: string; ext: string }> {
  const resp = await requestUrl({
    url,
    headers: { "User-Agent": "mymind/1.0" },
  });
  const ct = resp.headers["content-type"] || "";

  if (!ct.startsWith("image/")) {
    throw new Error(`Not an image (${ct})`);
  }

  if (resp.arrayBuffer.byteLength > 20 * 1024 * 1024) {
    throw new Error("Image too large (max 20MB)");
  }

  const base64 = arrayBufferToBase64(resp.arrayBuffer);
  const { mime, ext } = detectImageType(url, ct);

  return { base64, mime, ext };
}

async function extractFromURL(
  url: string
): Promise<{ title: string; text: string }> {
  const resp = await requestUrl({
    url,
    headers: { "User-Agent": "mymind/1.0" },
  });
  const parser = new DOMParser();
  const doc = parser.parseFromString(resp.text, "text/html");

  const title = doc.querySelector("title")?.textContent?.trim() || "";

  doc
    .querySelectorAll("script, style, nav, footer, aside")
    .forEach((el) => el.remove());

  let body = collapseWhitespace(doc.body?.textContent?.trim() || "");
  if (body.length > MAX_EXTRACT_CHARS) body = body.substring(0, MAX_EXTRACT_CHARS);

  return { title, text: body };
}

// ─── Gemini API ─────────────────────────────────────────────

async function analyzeWithGemini(
  settings: MymindSettings,
  content: ContentInput
): Promise<AIResult> {
  const apiUrl = `https://generativelanguage.googleapis.com/v1beta/models/${settings.model}:generateContent?key=${settings.apiKey}`;
  const prompt = buildPrompt(content, settings.prompt);

  let parts: any[];
  if (
    (content.kind === "image" || content.kind === "pdf") &&
    content.imageBase64
  ) {
    parts = [
      {
        inlineData: {
          mimeType: content.imageMIME,
          data: content.imageBase64,
        },
      },
      { text: prompt },
    ];
  } else {
    parts = [{ text: prompt }];
  }

  const body = {
    contents: [{ role: "user", parts }],
    generationConfig: {
      temperature: 0.3,
      responseMimeType: "application/json",
    },
  };

  const resp = await requestUrl({
    url: apiUrl,
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

  const text =
    resp.json?.candidates?.[0]?.content?.parts?.[0]?.text || "";
  return parseAIResult(text);
}

function buildPrompt(content: ContentInput, customPrompt: string): string {
  let sb = `Analyze the following content and return ONLY valid JSON with this exact schema:
{"title": "short descriptive title", "summary": "2-3 sentence summary"}

`;

  if (customPrompt) {
    sb += `Additional instructions: ${customPrompt}\n\n`;
  }

  switch (content.kind) {
    case "link":
      sb += `This is a web page. URL: ${content.source}\n\nContent:\n${content.text}`;
      break;
    case "image":
      sb += `This is an image from: ${content.source}\nAnalyze the image and generate a descriptive title and a summary of what the image shows.\n`;
      break;
    case "pdf":
      sb += `This is a PDF document from: ${content.source}\nAnalyze the PDF and generate a descriptive title and a summary of the document's content.\n`;
      break;
  }

  return sb;
}

function parseAIResult(text: string): AIResult {
  text = text.trim();
  const start = text.indexOf("{");
  const end = text.lastIndexOf("}");
  if (start !== -1 && end > start) {
    text = text.substring(start, end + 1);
  }

  try {
    const result = JSON.parse(text);
    return {
      title: result.title || "untitled",
      summary: result.summary || "(no summary)",
    };
  } catch {
    throw new Error("Could not parse AI response");
  }
}

// ─── Markdown Output ────────────────────────────────────────

function buildMarkdown(
  content: ContentInput,
  result: AIResult,
  imageName: string
): string {
  let md = `\n# ${result.title}\n\n`;

  if ((content.kind === "image" || content.kind === "pdf") && imageName) {
    md += `![[${imageName}]]\n\n`;
  }

  md += result.summary + "\n";

  if (content.source && content.source !== "clipboard") {
    md += `\n**Source:** ${content.source}\n`;
  }

  return md;
}

// ─── Clipboard ──────────────────────────────────────────────

async function readClipboardImage(): Promise<{ base64: string; mime: string }> {
  // Try Web Clipboard API first
  try {
    const items = await navigator.clipboard.read();
    for (const item of items) {
      for (const type of item.types) {
        if (type.startsWith("image/")) {
          const blob = await item.getType(type);
          const buffer = await blob.arrayBuffer();
          return { base64: arrayBufferToBase64(buffer), mime: type };
        }
      }
    }
  } catch {
    // Fall back to Electron clipboard
  }

  try {
    // @ts-ignore — Electron API available in desktop Obsidian
    const { clipboard } = require("electron");
    const image = clipboard.readImage();
    if (!image.isEmpty()) {
      return { base64: image.toPNG().toString("base64"), mime: "image/png" };
    }
  } catch {
    // ignore
  }

  throw new Error("No image in clipboard");
}

// ─── Utilities ──────────────────────────────────────────────

function slugify(s: string): string {
  s = s
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
  if (s.length > 50) {
    s = s.substring(0, 50).replace(/-$/, "");
  }
  return s;
}

function extFromMIME(mime: string): string {
  const map: Record<string, string> = {
    "image/png": ".png",
    "image/jpeg": ".jpg",
    "image/gif": ".gif",
    "image/webp": ".webp",
    "image/bmp": ".bmp",
  };
  return map[mime] || ".png";
}

function detectImageType(
  url: string,
  contentType: string
): { mime: string; ext: string } {
  try {
    const u = new URL(url);
    const ext = "." + u.pathname.split(".").pop()?.toLowerCase();
    if (ext in IMAGE_EXTENSIONS) return { mime: IMAGE_EXTENSIONS[ext], ext };
  } catch {
    /* ignore */
  }

  for (const [ext, mime] of Object.entries(IMAGE_EXTENSIONS)) {
    if (contentType.includes(mime)) return { mime, ext };
  }

  return { mime: "image/jpeg", ext: ".jpg" };
}

function collapseWhitespace(s: string): string {
  return s.replace(/\s+/g, " ").trim();
}

function arrayBufferToBase64(buffer: ArrayBuffer): string {
  let binary = "";
  const bytes = new Uint8Array(buffer);
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return window.btoa(binary);
}

function base64ToArrayBuffer(base64: string): ArrayBuffer {
  const binary = window.atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes.buffer;
}

// ─── Modal ──────────────────────────────────────────────────

class BookmarkURLModal extends Modal {
  plugin: MymindPlugin;

  constructor(plugin: MymindPlugin) {
    super(plugin.app);
    this.plugin = plugin;
  }

  onOpen() {
    const { contentEl } = this;
    contentEl.createEl("h3", { text: "Bookmark URL" });

    const input = contentEl.createEl("input", {
      type: "text",
      placeholder: "https://...",
    });
    input.style.width = "100%";
    input.style.marginBottom = "1em";

    const submit = async () => {
      const url = input.value.trim();
      if (!url) return;
      this.close();
      await this.plugin.bookmarkURL(url);
    };

    input.addEventListener("keydown", (e) => {
      if (e.key === "Enter") submit();
    });

    const btn = contentEl.createEl("button", {
      text: "Bookmark",
      cls: "mod-cta",
    });
    btn.addEventListener("click", submit);

    setTimeout(() => input.focus(), 10);
  }

  onClose() {
    this.contentEl.empty();
  }
}

// ─── Settings Tab ───────────────────────────────────────────

class MymindSettingTab extends PluginSettingTab {
  plugin: MymindPlugin;

  constructor(app: App, plugin: MymindPlugin) {
    super(app, plugin);
    this.plugin = plugin;
  }

  display() {
    const { containerEl } = this;
    containerEl.empty();

    new Setting(containerEl)
      .setName("Gemini API key")
      .setDesc("Your Google Gemini API key")
      .addText((text) =>
        text
          .setPlaceholder("Enter API key")
          .setValue(this.plugin.settings.apiKey)
          .onChange(async (value) => {
            this.plugin.settings.apiKey = value;
            await this.plugin.saveSettings();
          })
      );

    new Setting(containerEl)
      .setName("Model")
      .setDesc("Gemini model to use")
      .addText((text) =>
        text.setValue(this.plugin.settings.model).onChange(async (value) => {
          this.plugin.settings.model = value;
          await this.plugin.saveSettings();
        })
      );

    new Setting(containerEl)
      .setName("Custom prompt")
      .setDesc("Additional instructions for AI analysis")
      .addTextArea((text) =>
        text.setValue(this.plugin.settings.prompt).onChange(async (value) => {
          this.plugin.settings.prompt = value;
          await this.plugin.saveSettings();
        })
      );

    new Setting(containerEl)
      .setName("Bookmarks folder")
      .setDesc("Folder in vault for bookmarks")
      .addText((text) =>
        text
          .setValue(this.plugin.settings.bookmarksFolder)
          .onChange(async (value) => {
            this.plugin.settings.bookmarksFolder = value;
            await this.plugin.saveSettings();
          })
      );
  }
}
