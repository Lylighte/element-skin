# 阶段 11：收敛导入异常回显

## 目标

主改 `backends/profile_import_backend.py`：停止把原始异常文本 `str(e)` 直接回给调用方，避免泄露远端 URL、连接细节、内部错误信息。服务端记日志，对外返回通用文案。

## 问题证据

- `backends/profile_import_backend.py:38-39`：`raise HTTPException(400, detail=str(e))` 把底层异常原文回显。
- `profile_import_backend.py:99-102,134-139`：`failed[].detail = str(exc)` 把远端 Yggdrasil 客户端异常（含 URL、连接/内部错误）写进对调用方的响应。

## 设计决策

- 对外返回稳定的通用文案（如「导入失败」/「无法获取远端资料」），不含内部细节。
- 服务端 `logger.warning(..., exc_info=True)` 保留完整堆栈供排查。
- 区分**可告知用户的业务错误**（如「该角色名已存在」「验证码错误」——这些本就是给用户看的）与**内部异常**（连接、解析、未知错误——这些要收敛）。仅收敛后者，不要把有用的业务提示也一并抹掉。

## 改造清单

`backends/profile_import_backend.py`：

```python
import logging
logger = logging.getLogger(__name__)

# 单个导入失败处：
try:
    ...
except HTTPException:
    raise                      # 已是面向用户的业务错误，原样抛
except Exception as exc:
    logger.warning("profile import failed for %s", <safe_id>, exc_info=True)
    raise HTTPException(status_code=400, detail="导入失败，请稍后重试")

# 批量导入的 failed 列表项：
except HTTPException as he:
    failed.append({"id": item_id, "detail": he.detail})     # 业务错误可告知
except Exception:
    logger.warning("batch import item failed: %s", item_id, exc_info=True)
    failed.append({"id": item_id, "detail": "导入失败"})       # 内部错误收敛
```

> 实现时按现有代码结构对齐：保留对 `HTTPException` 的透传（那是有意给用户的提示），只把裸 `Exception → str(exc)` 收敛。日志中也避免记录敏感 URL 的 query 串（如含 token）。

## 影响文件

- `backends/profile_import_backend.py`（主）

## 验证

```bash
cd skin-backend
pytest tests/backends/test_profile_import_backend.py -q
pytest -q
```

针对性用例：

- 远端连接错误/解析错误 → 响应 `detail` 为通用文案，不含 URL/堆栈；日志中有完整记录。
- 业务错误（如角色名冲突）→ 仍向用户返回可读的业务提示。
