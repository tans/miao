# 创意喵视频稿件处理链路使用说明

## 一、整体流程

当前视频稿件的处理链路如下：

1. 小程序里，创作者先认领任务
2. 创作者进入“上传稿件”页选择视频
3. 小程序把原始视频上传到 `miao`
4. `miao` 把原始视频存入腾讯 COS
5. 创作者提交稿件
6. `miao` 为该视频创建处理任务，并调用 `miao_dataService`
7. `miao_dataService` 下载原视频
8. `miao_dataService` 用 `ffmpeg` 做压缩和加水印
9. `miao_dataService` 把处理后视频和封面图上传到 COS
10. `miao_dataService` 回调 `miao`
11. `miao` 更新稿件处理状态
12. 小程序显示“处理中”或可播放结果

## 二、各系统职责

### 1. 小程序 `miao-mini`

只负责：

- 认领任务
- 选择视频
- 上传视频到后端
- 提交稿件
- 展示处理状态

不负责：

- 直接访问 `miao_dataService`
- 直接上传处理后视频到 COS
- 本地压缩或加水印

### 2. 主后端 `miao`

负责：

- 接收小程序上传的视频
- 存原始视频到 COS
- 创建视频处理任务
- 调用 `miao_dataService`
- 接收处理回调
- 更新数据库中的稿件状态和视频地址
- 对小程序返回统一数据

### 3. 视频处理服务 `miao_dataService`

负责：

- 下载原始视频
- `ffmpeg` 压缩视频
- 添加水印
- 截取封面图
- 上传成品到 COS
- 回调 `miao`

### 4. 腾讯 COS

负责存储：

- 原始视频
- 处理后视频
- 封面图

## 三、正确使用流程

### 步骤 1：创作者先认领任务

在小程序任务详情页点击“报名/认领”。

说明：

- 这一步会真实调用后端创建 `claim`
- 如果没有先认领，后面上传稿件时会提示 `claim not found`

### 步骤 2：进入上传稿件页

认领成功后，再进入“上传稿件”页。

说明：

- 上传页会根据 `taskId` 去后端查当前用户的 `claim`
- 成功后会拿到真正的 `claim.id`

### 步骤 3：选择视频

创作者在上传页选择本地视频。

### 步骤 4：上传原始视频

小程序调用后端上传接口，把原视频先传给 `miao`。

当前逻辑：

- 小程序上传给 `miao`
- `miao` 直接把原视频写入 COS

原视频 COS 路径规范：

```text
claim-source/{claim_id}/{job_id}.*
```

例如：

```text
claim-source/123/claim-123-1713859200000.mp4
```

说明：

- 原视频保留原扩展名
- `job_id` 用于后续处理任务和成品文件对应

### 步骤 5：提交稿件

上传成功后，小程序调用：

```text
PUT /api/v1/creator/claim/:id/submit
```

提交的数据里包含：

- 文案描述
- 原始视频 URL
- 文件类型 `video`

### 步骤 6：`miao` 创建处理任务

`miao` 收到提交后：

- 保存素材记录
- 标记视频状态为 `pending/processing`
- 调用 `miao_dataService`

### 步骤 7：`miao_dataService` 处理视频

`miao_dataService` 会执行：

1. 下载 `source_url`
2. 使用 `ffmpeg` 处理视频
3. 压缩编码
4. 添加水印
5. 生成封面图
6. 上传结果到 COS

当前处理后存储路径规范：

处理后视频：

```text
claim-processed/{claim_id}/{job_id}.mp4
```

封面图：

```text
claim-processed/{claim_id}/{job_id}.jpg
```

### 步骤 8：处理完成回调 `miao`

处理完成后，`miao_dataService` 回调 `miao`，回传：

- `job_id`
- `status`
- `processed_url`
- `thumbnail_url`
- 错误信息（如果失败）

### 步骤 9：`miao` 更新状态

`miao` 更新数据库字段，例如：

- `source_file_path`
- `processed_file_path`
- `thumbnail_path`
- `process_status`
- `process_error`

### 步骤 10：小程序展示结果

小程序再查稿件/作品时：

- 未完成：显示“处理中”
- 失败：显示失败
- 完成：使用处理后视频播放

## 四、当前状态说明

当前已具备：

- 小程序上传原视频到 `miao`
- `miao` 保存原视频到 COS
- `miao` 调用 `miao_dataService`
- `miao_dataService` 压缩 + 加水印 + 生成封面
- `miao_dataService` 上传成品到 COS
- `miao_dataService` 回调 `miao`
- `miao` 更新状态
- 小程序显示处理状态

## 五、当前水印和压缩规则

当前 `miao_dataService` 使用 `ffmpeg`：

- 视频编码：`libx264`
- 音频编码：`aac`
- 压缩参数：`-preset veryfast -crf 28`
- 输出格式：`mp4`
- 封面格式：`jpg`

当前水印方式：

- `drawtext`
- 默认水印文字来自环境变量 `WATERMARK_TEXT`
- 未配置时默认值为 `miao`

## 六、依赖的环境变量

### `miao` 需要

```env
VIDEO_PROCESSING_SERVICE_URL
VIDEO_PROCESSING_CALLBACK_BASE_URL
VIDEO_PROCESSING_CALLBACK_SECRET
COS_APP_ID
COS_BUCKET
COS_REGION
COS_SECRET_ID
COS_SECRET_KEY
COS_CDN_HOST
JWT_SECRET
```

### `miao_dataService` 需要

可以直接复用 `/data/miao/.env`，重点包括：

```env
COS_APP_ID
COS_BUCKET
COS_REGION
COS_SECRET_ID
COS_SECRET_KEY
COS_CDN_HOST
WATERMARK_TEXT
CALLBACK_SECRET
FFMPEG_BIN
```

## 七、COS 配置说明

当前代码兼容两种配置方式。

方式 A：

```env
COS_APP_ID=1253811408
COS_BUCKET=clawos
```

方式 B：

```env
COS_BUCKET=clawos-1253811408
```

最终都会识别成：

```text
clawos-1253811408
```

## 八、常见问题

### 1. `claim not found`

原因：

- 没有先认领任务
- 或认领页只是前端假成功，没有真实调用后端

当前处理：

- 已修正任务详情页，认领会真实调用后端接口

正确流程：

- 先认领
- 再上传稿件

### 2. `任务信息不完整`

原因：

- 上传页之前错误使用了 `task.id`
- 实际提交需要的是 `claim.id`

当前处理：

- 已修正上传稿件页，单独保存并使用 `claimId`

### 3. `miao_dataService` 启动失败

常见原因：

- `COS_BUCKET` 没配
- `FFMPEG_BIN` 不存在
- 回调 secret 不匹配

### 4. `miao` 重启失败

常见原因：

- `JWT_SECRET` 没加载
- 现在 `restart.sh` 已改为自动加载 `/data/miao/.env`

## 九、当前启动方式

### `miao`

- 二进制：`/data/miao/miao-server`
- 重启：

```bash
cd /data/miao
./restart.sh
```

### `miao_dataService`

- 二进制：`/data/miao_dataService/miao_dataService`
- 重启：

```bash
cd /data/miao_dataService
./restart.sh
```

## 十、联调检查项

上传一次真实视频后，检查下面几项：

1. 小程序能成功认领任务
2. 上传稿件页能正常提交
3. COS 中出现原视频：

```text
claim-source/{claim_id}/{job_id}.*
```

4. COS 中出现处理结果：

```text
claim-processed/{claim_id}/{job_id}.mp4
claim-processed/{claim_id}/{job_id}.jpg
```

5. `miao_dataService` 日志有处理记录
6. `miao` 能收到回调
7. 小程序作品状态从“处理中”变成可播放
