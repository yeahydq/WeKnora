---
name: educomp-wrong-questions
description: 查询 EduComp 错题数据的只读技能。当用户要求按知识点读错题、查询某个学生的错题、按思维导图筛选错题、或分页查看错题时使用。支持从多个已配置的 EduComp URL 中选择 endpoint_id。
---

# EduComp Wrong Questions

通过 EduComp 的只读 API 查询错题数据。

## 适用场景

- 用户说“按知识点看错题”
- 用户说“查一下某个学生的错题”
- 用户说“按思维导图筛选错题”
- 用户说“分页列出错题”

## 输入约定

调用脚本时，使用 `execute_skill_script`，并把 JSON 放在 `input` 字段。

脚本路径：`scripts/query_wrong_questions.py`

输入 JSON 字段：

- `endpoint_id`: 可选，EduComp 地址标识，例如 `educomp-a`、`educomp-b`
- `knowledge_point`: 可选，知识点 ID 或标题，对应 EduComp 的 `knowledgePoint`
- `mind_map_id`: 可选，对应 EduComp 的 `mindMapId`
- `user_id`: 可选，对应 EduComp 的 `userId`
- `skip`: 可选，默认 `0`
- `take`: 可选，默认 `20`

示例：

```json
{
  "endpoint_id": "educomp-a",
  "knowledge_point": "一元一次方程",
  "skip": 0,
  "take": 20
}
```

## 工作流程

1. 先根据用户要求整理查询条件。
2. 调用 `execute_skill_script` 执行 `scripts/query_wrong_questions.py`。
3. 读取脚本输出中的 JSON 结果。
4. 用简洁中文总结：
   - 当前使用的 endpoint
   - 总数
   - 前几条错题标题或题干
   - 如果接口离线，明确提示是 EduComp 不可达

## 注意事项

- 这是只读技能，不能写入、修改或删除 EduComp 数据。
- 如果用户没有指定 `endpoint_id`，脚本会优先使用服务端默认地址；如果服务端未设置默认地址，则会使用已配置列表中的第一个地址。
- 如果返回为空，不要臆测，直接说明当前条件下没有查到错题。
