# Ralph Context Snapshot

## Task statement
修正产品对“创意”的表达和逻辑。用户明确指出这里的“创意”应指向“视频”，并要求先修正文案与文档，最后再把产品调整过来。

## Desired outcome
- 文案、说明、PRD 等文字材料与产品定位一致，明确平台核心是视频创意 / 视频内容交付。
- 产品中的任务分类、筛选、发布入口、任务详情等逻辑与该定位一致，不再把平台表达成泛创意任务市场。
- 变更后的行为有相应验证，避免把现有任务流打坏。

## Known facts / evidence
- [README.md](/Users/ke/code/miao/README.md) 已将产品定义为“视频创意任务平台”，商家端特性也写成“发布视频创意任务”。
- [docs/V1_0_PRD.md](/Users/ke/code/miao/docs/V1_0_PRD.md) 的 Task 模型仍保留 `1=文案, 2=设计, 3=视频, 4=摄影, 5=音乐, 6=开发, 7=其他`。
- [internal/model/task.go](/Users/ke/code/miao/internal/model/task.go) 把任务分类实现为 7 个常量，说明后端数据结构当前是多分类市场模型。
- [internal/handler/business.go](/Users/ke/code/miao/internal/handler/business.go) 的发单接口直接接收 `Category` 并保存，没有视频优先或单分类约束。
- [internal/handler/creator.go](/Users/ke/code/miao/internal/handler/creator.go) 的任务大厅接口支持 `category` 查询参数，说明产品流以分类筛选为既有能力。
- [web/templates/business/task_create.html](/Users/ke/code/miao/web/templates/business/task_create.html) 的商家发单页暴露 7 类任务选项。
- [web/templates/creator/task_hall.html](/Users/ke/code/miao/web/templates/creator/task_hall.html) 与 [web/templates/tasks.html](/Users/ke/code/miao/web/templates/tasks.html) 都把“任务分类”作为主要筛选维度。
- [web/templates/creator/task_detail.html](/Users/ke/code/miao/web/templates/creator/task_detail.html) 与 [web/templates/business/task_list.html](/Users/ke/code/miao/web/templates/business/task_list.html) 展示多分类标签。
- [test/test_task_filters.sh](/Users/ke/code/miao/test/test_task_filters.sh) 现有测试脚本也围绕多分类筛选编写。
- 工作区当前已有未提交变更：`README.md`、`web/templates/creator/task_hall.html`、`test/test_task_filters.sh` 等，后续修改需避免覆盖用户已有编辑。

## Constraints
- 当前处于 Ralph 工作流，需先完成上下文与规划，再进入实现。
- 任务是产品定位修正，不应在未明确产品边界前直接改模型或页面。
- 必须优先修正文案 / 文档，再做产品逻辑调整。
- 不可回滚或覆盖用户已有未提交修改。
- 不新增依赖。

## Unknowns / open questions
- “创意=视频”是要：
  - 彻底收敛成单一“视频”品类，删除其他分类；
  - 还是保留底层分类能力，但前台对外统一表述为视频创意，并把非视频项降级为内部扩展。
- 现有数据库中的非视频分类任务是否需要兼容显示 / 迁移。
- 文档修正的范围是否仅限仓库文档，还是也包括页面内用户可见文案。

## Likely touchpoints
- 文档：`README.md`、`docs/V1_0_PRD.md`、可能还有 `docs/开发计划.md` / `docs/development-guide.md`
- 模型与接口：`internal/model/task.go`、`internal/handler/business.go`、`internal/handler/creator.go`、`internal/repository/task.go`
- 前端模板：`web/templates/business/task_create.html`、`web/templates/creator/task_hall.html`、`web/templates/tasks.html`、`web/templates/creator/task_detail.html`、`web/templates/business/task_list.html`
- 测试：`test/test_task_filters.sh`

## Initial risk assessment
- 如果直接删分类，会牵动数据兼容、筛选测试、发单流程和详情展示，风险高于纯文案调整。
- 如果只改文案不改逻辑，会继续保留产品承诺与实际行为不一致的问题。
