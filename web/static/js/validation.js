// ========== 表单验证工具 ==========

// 必填验证
function validateRequired(value, fieldName = '此字段') {
  if (!value || value.toString().trim() === '') {
    return { valid: false, message: `${fieldName}不能为空` };
  }
  return { valid: true };
}

// 手机号验证
function validatePhone(phone) {
  if (!phone) {
    return { valid: false, message: '手机号不能为空' };
  }
  const phoneRegex = /^1[3-9]\d{9}$/;
  if (!phoneRegex.test(phone)) {
    return { valid: false, message: '请输入有效的手机号' };
  }
  return { valid: true };
}

// 邮箱验证
function validateEmail(email) {
  if (!email) {
    return { valid: false, message: '邮箱不能为空' };
  }
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  if (!emailRegex.test(email)) {
    return { valid: false, message: '请输入有效的邮箱地址' };
  }
  return { valid: true };
}

// 价格验证
function validatePrice(price, fieldName = '价格') {
  if (!price && price !== 0) {
    return { valid: false, message: `${fieldName}不能为空` };
  }
  const priceNum = parseFloat(price);
  if (isNaN(priceNum) || priceNum <= 0) {
    return { valid: false, message: `${fieldName}必须大于0` };
  }
  if (priceNum > 1000000) {
    return { valid: false, message: `${fieldName}不能超过1000000` };
  }
  return { valid: true };
}

// 整数验证
function validateInteger(value, fieldName = '数值', min = null, max = null) {
  if (!value && value !== 0) {
    return { valid: false, message: `${fieldName}不能为空` };
  }
  const num = parseInt(value);
  if (isNaN(num) || num.toString() !== value.toString()) {
    return { valid: false, message: `${fieldName}必须是整数` };
  }
  if (min !== null && num < min) {
    return { valid: false, message: `${fieldName}不能小于${min}` };
  }
  if (max !== null && num > max) {
    return { valid: false, message: `${fieldName}不能大于${max}` };
  }
  return { valid: true };
}

// 密码验证
function validatePassword(password) {
  if (!password) {
    return { valid: false, message: '密码不能为空' };
  }
  if (password.length < 6) {
    return { valid: false, message: '密码长度不能少于6位' };
  }
  if (password.length > 20) {
    return { valid: false, message: '密码长度不能超过20位' };
  }
  return { valid: true };
}

// 确认密码验证
function validatePasswordConfirm(password, confirmPassword) {
  if (!confirmPassword) {
    return { valid: false, message: '请确认密码' };
  }
  if (password !== confirmPassword) {
    return { valid: false, message: '两次密码输入不一致' };
  }
  return { valid: true };
}

// 文本长度验证
function validateLength(value, fieldName = '此字段', min = 0, max = null) {
  if (!value) {
    return { valid: false, message: `${fieldName}不能为空` };
  }
  const length = value.toString().length;
  if (length < min) {
    return { valid: false, message: `${fieldName}长度不能少于${min}个字符` };
  }
  if (max !== null && length > max) {
    return { valid: false, message: `${fieldName}长度不能超过${max}个字符` };
  }
  return { valid: true };
}

// 日期验证
function validateDate(dateStr, fieldName = '日期') {
  if (!dateStr) {
    return { valid: false, message: `${fieldName}不能为空` };
  }
  const date = new Date(dateStr);
  if (isNaN(date.getTime())) {
    return { valid: false, message: `${fieldName}格式不正确` };
  }
  return { valid: true };
}

// 未来日期验证
function validateFutureDate(dateStr, fieldName = '日期') {
  const result = validateDate(dateStr, fieldName);
  if (!result.valid) return result;

  const date = new Date(dateStr);
  const now = new Date();
  if (date <= now) {
    return { valid: false, message: `${fieldName}必须是未来时间` };
  }
  return { valid: true };
}

// 表单整体验证
function validateForm(formId, rules) {
  const form = document.getElementById(formId);
  if (!form) {
    console.error(`Form with id "${formId}" not found`);
    return false;
  }

  let isValid = true;
  const errors = [];

  for (const [fieldName, validators] of Object.entries(rules)) {
    const field = form.elements[fieldName];
    if (!field) {
      console.warn(`Field "${fieldName}" not found in form`);
      continue;
    }

    const value = field.value;

    for (const validator of validators) {
      const result = validator(value);
      if (!result.valid) {
        isValid = false;
        errors.push(result.message);

        // 添加错误样式
        field.classList.add('is-invalid');

        // 显示错误信息
        let feedback = field.nextElementSibling;
        if (!feedback || !feedback.classList.contains('invalid-feedback')) {
          feedback = document.createElement('div');
          feedback.className = 'invalid-feedback';
          field.parentNode.insertBefore(feedback, field.nextSibling);
        }
        feedback.textContent = result.message;

        break; // 只显示第一个错误
      } else {
        // 移除错误样式
        field.classList.remove('is-invalid');
        const feedback = field.nextElementSibling;
        if (feedback && feedback.classList.contains('invalid-feedback')) {
          feedback.remove();
        }
      }
    }
  }

  if (!isValid && errors.length > 0) {
    showError(errors[0]);
  }

  return isValid;
}

// 清除表单验证状态
function clearFormValidation(formId) {
  const form = document.getElementById(formId);
  if (!form) return;

  const invalidFields = form.querySelectorAll('.is-invalid');
  invalidFields.forEach(field => {
    field.classList.remove('is-invalid');
    const feedback = field.nextElementSibling;
    if (feedback && feedback.classList.contains('invalid-feedback')) {
      feedback.remove();
    }
  });
}

// 实时验证绑定
function bindRealtimeValidation(formId, rules) {
  const form = document.getElementById(formId);
  if (!form) return;

  for (const [fieldName, validators] of Object.entries(rules)) {
    const field = form.elements[fieldName];
    if (!field) continue;

    field.addEventListener('blur', () => {
      const value = field.value;
      for (const validator of validators) {
        const result = validator(value);
        if (!result.valid) {
          field.classList.add('is-invalid');
          let feedback = field.nextElementSibling;
          if (!feedback || !feedback.classList.contains('invalid-feedback')) {
            feedback = document.createElement('div');
            feedback.className = 'invalid-feedback';
            field.parentNode.insertBefore(feedback, field.nextSibling);
          }
          feedback.textContent = result.message;
          break;
        } else {
          field.classList.remove('is-invalid');
          const feedback = field.nextElementSibling;
          if (feedback && feedback.classList.contains('invalid-feedback')) {
            feedback.remove();
          }
        }
      }
    });

    field.addEventListener('input', () => {
      if (field.classList.contains('is-invalid')) {
        field.classList.remove('is-invalid');
        const feedback = field.nextElementSibling;
        if (feedback && feedback.classList.contains('invalid-feedback')) {
          feedback.remove();
        }
      }
    });
  }
}
