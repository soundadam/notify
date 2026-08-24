# notify

**实验性、已冻结。** [njuwatch](https://github.com/soundadam/nju-seat-watch) 第一期只需本机告警，**不依赖**本仓库。把外人放进 Keycloak、再用 `soundadam-scli` JWT 走 Resend，不是下一期方向；经 scli 的身份接入因此搁置。

HTTP 服务代码仍保留（契约见下），只是不要把它当成 njuwatch 的必选路径。本仓库不轮询学校站点，也不做验证码。

以后若要做邮件通道：先验证邮箱再签发令牌即可，不必经过 Keycloak。当前不实现。

---

集群内薄 HTTP 服务：校验 **soundadam-scli** 的 Keycloak access token，用 Resend 发信到 JWT `email`。不轮询学校站点，不保存学校凭据，不接受请求体里的收件人。

对外路径（由 Envoy `/notify` 前缀改写）：

```
POST https://api.soundadam.com/notify/v1/alerts
Authorization: Bearer <soundadam-scli access token>
Content-Type: application/json

{"title":"...","body":"...","dedupeKey":"<sub>:<id>"}

GET https://api.soundadam.com/notify/healthz
```

容器内监听 `:8080`：`POST /v1/alerts`、`GET /healthz`。

## 契约

- 收件人只取 JWT `email`；没有该 claim → **403**。请求体里的 `to` 会被忽略。
- 无/坏 token、错误 `iss`、`azp` 不是 `soundadam-scli` → **401**。
- 同一 `dedupeKey` 默认 **30 分钟**内存去重；冷却命中仍 **200**（避免客户端重试风暴）。
- From 固定 `soundadam <no-reply@soundadam.com>`。

## 环境变量

| 变量 | 必填 | 默认 | 说明 |
| --- | --- | --- | --- |
| `RESEND_API_KEY` | 是 | — | 只从环境读，不要写入仓库 |
| `OIDC_ISSUER` | 否 | `https://auth.soundadam.com/realms/soundadam` | 校验 `iss` |
| `OIDC_JWKS_URL` | 否 | 由 issuer 推导 certs | 集群内设 `http://keycloak.keycloak.svc.cluster.local:8080/realms/soundadam`（realm 基址即可，服务会补 `/protocol/openid-connect/certs`） |
| `OIDC_ALLOWED_AZP` | 否 | `soundadam-scli` | 允许的 `azp` |
| `LISTEN_ADDR` | 否 | `:8080` | |
| `DEDUPE_TTL` | 否 | `30m` | Go duration |
| `MAIL_FROM` | 否 | `soundadam <no-reply@soundadam.com>` | |
| `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY` | 集群 | — | 标准库默认尊重。出站 Resend 走 egress；JWKS 应 `NO_PROXY` 掉 in-cluster Keycloak |

## 镜像

GitHub Actions 推送到 `ghcr.io/soundadam/notify`（linux/amd64 + linux/arm64）。GitOps **钉 digest**，例如：

```text
ghcr.io/soundadam/notify@sha256:<digest>
```

也可参考 tag `ghcr.io/soundadam/notify:<git-sha>`，但部署以 digest 为准。

## 本地

```bash
go test ./...
RESEND_API_KEY=re_test go run ./cmd/notify
```

单测使用假 JWKS 与假 Resend，不会发真实邮件。
