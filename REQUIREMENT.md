Yêu cầu chi tiết
1. Kiến trúc Action

Dạng composite action (dùng action.yml).

Có thư mục scripts/ chứa các shell script chính:

detect-language.sh → phát hiện ngôn ngữ/framework.

run-review.sh → chạy linter + reviewdog theo ngôn ngữ.

Hỗ trợ chạy trên ubuntu-latest.

2. Cài đặt reviewdog

Tự động tải và cài reviewdog (phiên bản stable).

Đảm bảo add vào $PATH.

3. Logic detect ngôn ngữ

Node.js/TS → package.json

Python → requirements.txt hoặc pyproject.toml

Java → pom.xml hoặc build.gradle

Go → go.mod

.NET → file *.csproj

Nếu không nhận diện được → log warning và skip.

4. Tool cho từng ngôn ngữ

Node.js/TS: eslint (format stylish → reviewdog reporter eslint).

Python: flake8 + bandit (security).

Java: mvn checkstyle:check.

Go: go vet + golint.

.NET: dotnet format --verify-no-changes.

5. Reviewdog integration

Kết quả linter pipe sang reviewdog.

Config reviewdog:

Reporter: github-pr-review

Fail on error: true

Name: tương ứng tool (eslint, flake8, bandit, checkstyle, govet, dotnet-format).

Token: dùng ${{ inputs.github_token }} (định nghĩa trong action.yml).

6. action.yml

Input:

github_token: bắt buộc, để reviewdog post comment.

Runs:

Gọi shell script detect-language.sh và run-review.sh