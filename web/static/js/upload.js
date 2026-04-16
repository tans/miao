// ========== 文件上传工具 ==========

class FileUploader {
  constructor(options = {}) {
    this.inputId = options.inputId || 'file-input';
    this.previewId = options.previewId || 'file-preview';
    this.dropZoneId = options.dropZoneId || 'drop-zone';
    this.fileType = options.fileType || 'image'; // 'image' or 'video'

    // 根据文件类型设置默认值
    if (this.fileType === 'video') {
      this.maxSize = options.maxSize || 500 * 1024 * 1024; // 500MB
      this.allowedTypes = options.allowedTypes || [
        'video/mp4', 'video/quicktime', 'video/x-msvideo',
        'video/x-ms-wmv', 'video/x-flv', 'video/x-matroska', 'video/webm'
      ];
    } else {
      this.maxSize = options.maxSize || 5 * 1024 * 1024; // 5MB
      this.allowedTypes = options.allowedTypes || ['image/jpeg', 'image/png', 'image/gif', 'image/webp'];
    }

    this.multiple = options.multiple || false;
    this.onFileSelect = options.onFileSelect || null;
    this.files = [];

    this.init();
  }

  init() {
    const input = document.getElementById(this.inputId);
    const dropZone = document.getElementById(this.dropZoneId);

    if (input) {
      input.addEventListener('change', (e) => this.handleFileSelect(e));
    }

    if (dropZone) {
      dropZone.addEventListener('dragover', (e) => this.handleDragOver(e));
      dropZone.addEventListener('dragleave', (e) => this.handleDragLeave(e));
      dropZone.addEventListener('drop', (e) => this.handleDrop(e));
      dropZone.addEventListener('click', () => {
        if (input) input.click();
      });
    }
  }

  handleFileSelect(e) {
    const files = Array.from(e.target.files);
    this.processFiles(files);
  }

  handleDragOver(e) {
    e.preventDefault();
    e.stopPropagation();
    const dropZone = document.getElementById(this.dropZoneId);
    if (dropZone) {
      dropZone.classList.add('drag-over');
    }
  }

  handleDragLeave(e) {
    e.preventDefault();
    e.stopPropagation();
    const dropZone = document.getElementById(this.dropZoneId);
    if (dropZone) {
      dropZone.classList.remove('drag-over');
    }
  }

  handleDrop(e) {
    e.preventDefault();
    e.stopPropagation();
    const dropZone = document.getElementById(this.dropZoneId);
    if (dropZone) {
      dropZone.classList.remove('drag-over');
    }

    const files = Array.from(e.dataTransfer.files);
    this.processFiles(files);
  }

  processFiles(files) {
    if (!this.multiple && files.length > 1) {
      showError('只能上传一个文件');
      return;
    }

    const validFiles = [];
    for (const file of files) {
      const validation = this.validateFile(file);
      if (validation.valid) {
        validFiles.push(file);
      } else {
        showError(validation.message);
      }
    }

    if (validFiles.length > 0) {
      if (this.multiple) {
        this.files = [...this.files, ...validFiles];
      } else {
        this.files = [validFiles[0]];
      }
      this.renderPreview();
      if (this.onFileSelect) {
        this.onFileSelect(this.files);
      }
    }
  }

  validateFile(file) {
    // 检查文件类型
    if (!this.allowedTypes.includes(file.type)) {
      return {
        valid: false,
        message: `不支持的文件类型: ${file.type}。允许的类型: ${this.allowedTypes.join(', ')}`
      };
    }

    // 检查文件大小
    if (file.size > this.maxSize) {
      const maxSizeMB = (this.maxSize / 1024 / 1024).toFixed(1);
      return {
        valid: false,
        message: `文件大小超过限制 (最大 ${maxSizeMB}MB)`
      };
    }

    return { valid: true };
  }

  renderPreview() {
    const preview = document.getElementById(this.previewId);
    if (!preview) return;

    preview.innerHTML = '';

    this.files.forEach((file, index) => {
      const previewItem = document.createElement('div');
      previewItem.className = 'preview-item position-relative d-inline-block m-2';
      previewItem.style.width = '150px';

      if (file.type.startsWith('image/')) {
        const reader = new FileReader();
        reader.onload = (e) => {
          previewItem.innerHTML = `
            <img src="${e.target.result}" class="img-thumbnail" alt="${file.name}">
            <button type="button" class="btn btn-sm btn-danger position-absolute top-0 end-0 m-1" onclick="fileUploader.removeFile(${index})">
              <i class="bi bi-x"></i> ×
            </button>
            <div class="small text-center mt-1 text-truncate">${file.name}</div>
            <div class="small text-center text-muted">${this.formatFileSize(file.size)}</div>
          `;
        };
        reader.readAsDataURL(file);
      } else if (file.type.startsWith('video/')) {
        const videoUrl = URL.createObjectURL(file);
        previewItem.innerHTML = `
          <video class="img-thumbnail" style="width: 100%; height: 150px; object-fit: cover;" controls>
            <source src="${videoUrl}" type="${file.type}">
          </video>
          <button type="button" class="btn btn-sm btn-danger position-absolute top-0 end-0 m-1" onclick="fileUploader.removeFile(${index})">
            <i class="bi bi-x"></i> ×
          </button>
          <div class="small text-center mt-1 text-truncate">${file.name}</div>
          <div class="small text-center text-muted">${this.formatFileSize(file.size)}</div>
        `;
      } else {
        previewItem.innerHTML = `
          <div class="border rounded p-3 text-center">
            <div class="mb-2">📄</div>
            <div class="small text-truncate">${file.name}</div>
            <div class="small text-muted">${this.formatFileSize(file.size)}</div>
            <button type="button" class="btn btn-sm btn-danger mt-2" onclick="fileUploader.removeFile(${index})">删除</button>
          </div>
        `;
      }

      preview.appendChild(previewItem);
    });
  }

  removeFile(index) {
    this.files.splice(index, 1);
    this.renderPreview();
    if (this.onFileSelect) {
      this.onFileSelect(this.files);
    }
  }

  formatFileSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }

  getFiles() {
    return this.files;
  }

  clear() {
    this.files = [];
    this.renderPreview();
    const input = document.getElementById(this.inputId);
    if (input) input.value = '';
  }

  async uploadFiles(uploadUrl, additionalData = {}) {
    if (this.files.length === 0) {
      showError('请选择文件');
      return null;
    }

    try {
      showLoading('上传中...');

      const uploadedUrls = [];

      // 逐个上传文件
      for (const file of this.files) {
        const formData = new FormData();
        formData.append('file', file);

        // 添加额外数据
        for (const [key, value] of Object.entries(additionalData)) {
          formData.append(key, value);
        }

        const token = localStorage.getItem('token');
        const response = await fetch(`${uploadUrl}?type=${this.fileType}`, {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${token}`
          },
          body: formData
        });

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }

        const result = await response.json();

        if (result.code === 0) {
          uploadedUrls.push(result.data.url);
        } else {
          throw new Error(result.message || '上传失败');
        }
      }

      hideLoading();
      showSuccess('上传成功');

      return {
        urls: uploadedUrls,
        count: uploadedUrls.length
      };
    } catch (error) {
      hideLoading();
      console.error('Upload error:', error);
      showError(error.message || '上传失败，请稍后重试');
      return null;
    }
  }
}

// 全局实例（可选）
let fileUploader = null;

// 初始化上传器的辅助函数
function initFileUploader(options) {
  fileUploader = new FileUploader(options);
  return fileUploader;
}
