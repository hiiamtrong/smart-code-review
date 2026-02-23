# Code Quality Pre-commit Hook - Hướng dẫn cài đặt

> **NỘI BỘ** - Không chia sẻ tài liệu này ra ngoài công ty.

---

## Yêu cầu

- **Git** (Windows: cài [Git for Windows](https://git-scm.com/download/win) — đi kèm Git Bash)
- **Java 17+** (cần cho SonarQube scanner)
- **Terminal**: Git Bash (Windows) / Terminal (macOS) / Bash (Linux)

### Cài Java 17

<details>
<summary><b>macOS</b></summary>

```bash
brew install openjdk@17
```
</details>

<details>
<summary><b>Linux (Ubuntu/Debian)</b></summary>

```bash
sudo apt update && sudo apt install openjdk-17-jdk
```
</details>

<details>
<summary><b>Windows</b></summary>

```powershell
winget install EclipseAdoptium.Temurin.17.JDK
```

Hoặc tải từ [Adoptium](https://adoptium.net/temurin/releases/?version=17) → cài → tick **"Add to PATH"**.

> Sau khi cài xong, **đóng tất cả terminal rồi mở lại**.
</details>

Verify: `java -version`

---

## Cài đặt (chỉ cần làm 1 lần)

### Bước 1: Cài tool

**macOS / Linux / WSL:**
```bash
curl -sSL https://raw.githubusercontent.com/hiiamtrong/smart-code-review/main/scripts/local/install.sh | bash
```

**Windows (Git Bash):**
```bash
curl -sSL https://raw.githubusercontent.com/hiiamtrong/smart-code-review/main/scripts/local/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/hiiamtrong/smart-code-review/main/scripts/local/install.ps1 | iex
```

> Restart terminal sau khi cài.

### Bước 2: Cấu hình

```bash
ai-review setup
```

Điền theo bảng:

| Prompt                      | Giá trị                                        |
| --------------------------- | ---------------------------------------------- |
| Enable AI Review?           | `n`                                            |
| Enable SonarQube?           | `y`                                            |
| SonarQube Host URL          | `https://sonarqube.sotatek.works`              |
| SonarQube Token             | `sqa_e681b3c3e7107cc587b1f430ceaa9fbf129fe26d` |
| Block on Security Hotspots? | `y`                                            |

### Bước 3: Cài vào dự án

```bash
cd /path/to/du-an
ai-review install
```

- **SonarQube Project Key**: chọn theo dự án:
  - Backend: `SLive-BE`
  - Frontend: `SLive-FE`
- **Base Branch**: Enter (auto-detect `main` hoặc `master`)

**Done!**

---

## Sử dụng

Commit bình thường — hook tự chạy:

```bash
git add .
git commit -m "feat: add feature"
```

```
────────────────────────────────────────
  STEP 1/1: SonarQube Static Analysis
────────────────────────────────────────

[INFO] Project: my-project → https://sonarqube.xxx
[INFO] Scanning 3 changed file(s)...
[OK] SonarQube: No issues found
```

Nếu có lỗi → **commit bị block** → fix lỗi rồi commit lại.

### Bypass (khẩn cấp)

```bash
git commit --no-verify -m "hotfix"
```

> Hạn chế dùng. Lệnh này skip toàn bộ check.

---

## Tùy chỉnh

```bash
# Tắt block security hotspot
ai-review config set SONAR_BLOCK_ON_HOTSPOTS false

# Xem tất cả issues (kể cả dòng không thay đổi)
ai-review config set SONAR_FILTER_CHANGED_LINES_ONLY false

# Xem config hiện tại
ai-review config show

# Gỡ hook khỏi dự án
ai-review uninstall

# Update tool lên bản mới
ai-review update
```

---

## Exclude files không cần scan

Tạo `.sonarignore` trong project root:

```gitignore
.idea/
.vscode/
*.min.js
*.bundle.js
dist/
build/
node_modules/
vendor/
docs/
*.md
```

> Thêm `.sonarignore` vào `.gitignore` nếu không muốn commit.

---

## Lưu ý bảo mật

| Gì                   | Ở đâu                  | Có bị commit?                      |
| -------------------- | ---------------------- | ---------------------------------- |
| Pre-commit hook      | `.git/hooks/`          | Không                              |
| Config & token       | `~/.config/ai-review/` | Không                              |
| SonarQube temp files | `.scannerwork/`        | Hook tự dọn                        |
| `.sonarignore`       | Project root           | Có — thêm vào `.gitignore` nếu cần |

---

## Troubleshooting

| Lỗi                                  | Nguyên nhân                                                     | Fix                                      |
| ------------------------------------ | --------------------------------------------------------------- | ---------------------------------------- |
| `java: command not found`            | Chưa cài Java                                                   | Cài Java 17 (xem trên), restart terminal |
| `You're not authorized`              | Project key sai hoặc token hết hạn                              | Kiểm tra lại key và token với team lead  |
| Hook không chạy                      | Chưa install hook                                               | `ai-review install`                      |
| Commit quá chậm                      | Scan nhiều file                                                 | Commit nhỏ hơn, thêm `.sonarignore`      |
| `unzip: command not found` (Windows) | Tool tự fallback PowerShell, nếu vẫn lỗi: `scoop install unzip` |
| Colors bị lỗi (Windows)              | Dùng **Windows Terminal** hoặc **Git Bash** thay cmd.exe        |
