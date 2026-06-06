---
name: educomp-knowledge-points
description: 查询 EduComp 知识点数据的只读技能。当用户要求查看知识点列表、知识点树、思维导图下的知识点、或读取某个知识点下的错题时使用。支持从多个已配置的 EduComp URL 中选择 endpoint_id。
---

# EduComp Knowledge Points

通过 EduComp 的只读 API 查询知识点与知识点树。

## 适用场景

- 用户说“列一下知识点”
- 用户说“查看知识点树”
- 用户说“查某个思维导图下面有哪些知识点”
- 用户说“读取某个知识点下面的错题”

## 输入约定

调用脚本时，使用 `execute_skill_script`，并把 JSON 放在 `input` 字段。

脚本路径：`scripts/query_knowledge_points.py`

输入 JSON 字段：

- `endpoint_id`: 可选，EduComp 地址标识，例如 `educomp-a`
- `mode`: 必填，可选值：`list`、`tree`、`mindmap`、`wrong_questions`
- `subject`: 可选，学科过滤
- `mind_map_id`: `mode=mindmap` 时可选
- `knowledge_point_id`: `mode=wrong_questions` 时必填
- `user_id`: 可选
- `skip`: 可选，默认 `0`
- `take`: 可选，默认 `50`

示例：

```json
{
  "endpoint_id": "educomp-a",
  "mode": "wrong_questions",
  "knowledge_point_id": "ckp1234567890",
  "skip": 0,
  "take": 20
}
```

## 工作流程

1. 判断用户需要的是列表、树、思维导图知识点，还是知识点下错题。
2. 调用 `scripts/query_knowledge_points.py`。
3. 如果拿到的是树结构，先概括主要层级，不要原样倾倒整棵树。
4. 如果拿到的是知识点下错题，优先总结题目数量和代表性题目。

## 注意事项

- 只做查询，不做写入。
- `mode=wrong_questions` 时必须提供 `knowledge_point_id`。
- 如果上游离线，明确提示 EduComp 不在线，而不是笼统说“系统错误”。
