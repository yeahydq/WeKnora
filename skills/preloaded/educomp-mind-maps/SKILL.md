---
name: educomp-mind-maps
description: 查询 EduComp 思维导图与导图知识点的只读技能。当用户要求搜索思维导图、查看有错题的思维导图、按学科查看思维导图，或读取导图知识点列表时使用。支持从多个已配置的 EduComp URL 中选择 endpoint_id。
---

# EduComp Mind Maps

通过 EduComp 的只读 API 查询思维导图数据。

## 适用场景

- 用户说“查思维导图”
- 用户说“看数学思维导图”
- 用户说“只看有错题的思维导图”
- 用户说“列出导图知识点”

## 输入约定

调用脚本时，使用 `execute_skill_script`，并把 JSON 放在 `input` 字段。

脚本路径：`scripts/query_mind_maps.py`

输入 JSON 字段：

- `endpoint_id`: 可选，EduComp 地址标识，例如 `educomp-a`
- `mode`: 可选，`list` 或 `knowledge_points_list`，默认 `list`
- `q`: 可选，思维导图搜索关键词
- `subject`: 可选，学科过滤
- `user_id`: 可选，指定用户
- `has_wrong_questions`: 可选，布尔值
- `skip`: 可选，默认 `0`
- `take`: 可选，默认 `20`

示例：

```json
{
  "endpoint_id": "educomp-a",
  "mode": "list",
  "subject": "数学",
  "has_wrong_questions": true,
  "skip": 0,
  "take": 20
}
```

## 工作流程

1. 判断是查思维导图列表，还是查导图知识点扁平列表。
2. 调用 `scripts/query_mind_maps.py`。
3. 总结时优先输出标题、学科、数量和是否有错题，不要原样输出超长 JSON。

## 注意事项

- 只做只读查询。
- 如果用户想进一步按知识点读错题，优先改用 `educomp-knowledge-points` 或 `educomp-wrong-questions` 技能。
