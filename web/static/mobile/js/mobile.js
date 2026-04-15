// 创意喵移动端 JavaScript

// 初始化底部导航栏
function initTabBar() {
    const tabItems = document.querySelectorAll('.mobile-tab-item');

    tabItems.forEach(item => {
        item.addEventListener('click', function(e) {
            // 允许默认导航行为
            // 服务器端已经设置了 active 状态，无需客户端重复处理
        });
    });
}

// API 请求封装
// TODO: Implement CSRF protection with backend middleware
async function apiRequest(url, options = {}) {
    const token = localStorage.getItem('token');

    const defaultOptions = {
        headers: {
            'Content-Type': 'application/json',
            ...(token && { 'Authorization': `Bearer ${token}` })
        }
    };

    const mergedOptions = {
        ...defaultOptions,
        ...options,
        headers: {
            ...defaultOptions.headers,
            ...options.headers
        }
    };

    try {
        const response = await fetch(url, mergedOptions);
        const data = await response.json();

        // 处理未授权
        if (response.status === 401) {
            localStorage.removeItem('token');
            showToast('请先登录', 'error');
            setTimeout(() => {
                window.location.href = '/mobile/login';
            }, 1500);
            return null;
        }

        // 处理权限不足
        if (response.status === 403) {
            showToast('权限不足', 'error');
            return null;
        }

        // 处理资源不存在
        if (response.status === 404) {
            showToast('资源不存在', 'error');
            return null;
        }

        // 处理服务器错误
        if (response.status >= 500) {
            showToast('服务器错误，请稍后重试', 'error');
            return null;
        }

        // 处理业务错误
        if (!response.ok || data.code !== 0) {
            throw new Error(data.message || '请求失败');
        }

        return data;
    } catch (error) {
        console.error('API Request Error:', error);
        showToast(error.message || '网络错误', 'error');
        return null;
    }
}

// 显示加载指示器
function showLoading(text = '加载中...') {
    let loading = document.querySelector('.mobile-loading');
    if (!loading) {
        loading = document.createElement('div');
        loading.className = 'mobile-loading';
        loading.innerHTML = '<div class="spinner"></div><div class="mobile-loading-text">加载中...</div>';
        document.body.appendChild(loading);
    }
    loading.querySelector('.mobile-loading-text').textContent = text;
    loading.style.display = 'block';
}

// 隐藏加载指示器
function hideLoading() {
    const loading = document.querySelector('.mobile-loading');
    if (loading) {
        loading.style.display = 'none';
    }
}

// 显示 Toast 提示
function showToast(message, type = 'info') {
    let toast = document.querySelector('.mobile-toast');
    if (!toast) {
        toast = document.createElement('div');
        toast.className = 'mobile-toast';
        document.body.appendChild(toast);
    }

    toast.textContent = message;
    toast.className = `mobile-toast ${type}`;
    toast.classList.add('show');

    setTimeout(() => {
        toast.classList.remove('show');
    }, 3000);
}

// 退出登录
function logout() {
    localStorage.removeItem('token');
    localStorage.removeItem('user_id');
    localStorage.removeItem('username');
    localStorage.removeItem('user');
    showToast('已退出登录', 'success');
    setTimeout(() => {
        window.location.href = '/mobile/login';
    }, 1000);
}

// 无限滚动
function initInfiniteScroll(loadMoreCallback) {
    let loading = false;
    let hasMore = true;

    window.addEventListener('scroll', async () => {
        if (loading || !hasMore) return;

        const scrollTop = window.pageYOffset || document.documentElement.scrollTop;
        const windowHeight = window.innerHeight;
        const documentHeight = document.documentElement.scrollHeight;

        // 距离底部 100px 时触发加载
        if (scrollTop + windowHeight >= documentHeight - 100) {
            loading = true;
            showLoading();

            try {
                const result = await loadMoreCallback();
                hasMore = result && result.hasMore;
            } catch (error) {
                console.error('Load more error:', error);
                showToast('加载失败', 'error');
            } finally {
                loading = false;
                hideLoading();
            }
        }
    });
}

// 格式化金额
function formatMoney(amount) {
    return `¥${parseFloat(amount).toFixed(2)}`;
}

// 格式化日期
function formatDate(dateString) {
    const date = new Date(dateString);
    const now = new Date();
    const diff = now - date;

    // 1分钟内
    if (diff < 60000) {
        return '刚刚';
    }
    // 1小时内
    if (diff < 3600000) {
        return `${Math.floor(diff / 60000)}分钟前`;
    }
    // 24小时内
    if (diff < 86400000) {
        return `${Math.floor(diff / 3600000)}小时前`;
    }
    // 7天内
    if (diff < 604800000) {
        return `${Math.floor(diff / 86400000)}天前`;
    }

    // 超过7天显示日期
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');

    if (year === now.getFullYear()) {
        return `${month}-${day}`;
    }
    return `${year}-${month}-${day}`;
}

// 检查登录状态
function checkAuth() {
    const token = localStorage.getItem('token');
    if (!token) {
        showToast('请先登录', 'error');
        setTimeout(() => {
            window.location.href = '/mobile/login';
        }, 1500);
        return false;
    }
    return true;
}

// 获取用户信息
async function getUserInfo() {
    const data = await apiRequest('/api/v1/users/me');
    if (data && data.data) {
        return data.data;
    }
    return null;
}

// 获取本地缓存的用户信息
function getUser() {
    const userStr = localStorage.getItem('user');
    if (userStr) {
        try {
            return JSON.parse(userStr);
        } catch (e) {
            return null;
        }
    }
    return null;
}

// 检查是否已登录
function isLoggedIn() {
    return !!localStorage.getItem('token');
}

// 图片懒加载
function initLazyLoad() {
    const images = document.querySelectorAll('img[data-src]');

    const imageObserver = new IntersectionObserver((entries, observer) => {
        entries.forEach(entry => {
            if (entry.isIntersecting) {
                const img = entry.target;
                img.src = img.dataset.src;
                img.removeAttribute('data-src');
                observer.unobserve(img);
            }
        });
    });

    images.forEach(img => imageObserver.observe(img));
}

// 防抖函数
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// 节流函数
function throttle(func, limit) {
    let inThrottle;
    return function(...args) {
        if (!inThrottle) {
            func.apply(this, args);
            inThrottle = true;
            setTimeout(() => inThrottle = false, limit);
        }
    };
}

// Pull-to-refresh functionality
function initPullToRefresh(refreshCallback) {
    let startY = 0;
    let currentY = 0;
    let isPulling = false;
    const threshold = 80;
    const maxPull = 120;

    let refreshIndicator = document.querySelector('.pull-refresh-indicator');
    if (!refreshIndicator) {
        refreshIndicator = document.createElement('div');
        refreshIndicator.className = 'pull-refresh-indicator';
        refreshIndicator.innerHTML = '<div class="pull-refresh-spinner"></div><div class="pull-refresh-text">下拉刷新</div>';
        document.body.insertBefore(refreshIndicator, document.body.firstChild);
    }

    document.addEventListener('touchstart', (e) => {
        if (window.scrollY === 0) {
            startY = e.touches[0].clientY;
            isPulling = true;
        }
    }, { passive: true });

    document.addEventListener('touchmove', (e) => {
        if (!isPulling) return;

        currentY = e.touches[0].clientY;
        const distance = currentY - startY;

        if (distance > 0 && window.scrollY === 0) {
            const pullDistance = Math.min(distance * 0.5, maxPull);
            refreshIndicator.style.transform = `translateY(${pullDistance}px)`;
            refreshIndicator.style.opacity = Math.min(pullDistance / threshold, 1);

            if (pullDistance >= threshold) {
                refreshIndicator.querySelector('.pull-refresh-text').textContent = '释放刷新';
            } else {
                refreshIndicator.querySelector('.pull-refresh-text').textContent = '下拉刷新';
            }
        }
    }, { passive: true });

    document.addEventListener('touchend', async (e) => {
        if (!isPulling) return;

        const distance = currentY - startY;
        const pullDistance = Math.min(distance * 0.5, maxPull);

        if (pullDistance >= threshold) {
            refreshIndicator.querySelector('.pull-refresh-text').textContent = '刷新中...';
            refreshIndicator.classList.add('refreshing');

            try {
                await refreshCallback();
            } finally {
                setTimeout(() => {
                    refreshIndicator.style.transform = 'translateY(0)';
                    refreshIndicator.style.opacity = '0';
                    refreshIndicator.classList.remove('refreshing');
                }, 300);
            }
        } else {
            refreshIndicator.style.transform = 'translateY(0)';
            refreshIndicator.style.opacity = '0';
        }

        isPulling = false;
        startY = 0;
        currentY = 0;
    }, { passive: true });
}

// HTML escaping to prevent XSS
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Constants
const TASKS_PER_PAGE = 20;

// Load tasks from API
async function loadTasks(page = 1, category = '', keyword = '') {
    const params = new URLSearchParams({
        page: page,
        limit: TASKS_PER_PAGE,
        sort: 'created_at'
    });

    if (keyword) params.append('keyword', keyword);
    if (category) params.append('type', category);

    const data = await apiRequest(`/api/v1/creator/tasks?${params}`);
    return data;
}

// Initialize task hall page
function initTaskHall() {
    const taskList = document.getElementById('taskList');
    const searchInput = document.getElementById('searchInput');
    const categoryTabs = document.querySelectorAll('.mobile-category-tabs .mobile-tag');
    const loadingMore = document.getElementById('loadingMore');
    const emptyState = document.getElementById('emptyState');

    if (!taskList) return; // Not on task hall page

    let currentPage = 1;
    let currentCategory = '';
    let currentKeyword = '';
    let isLoading = false;
    let hasMore = true;

    // Category filter
    categoryTabs.forEach(tab => {
        tab.addEventListener('click', function() {
            categoryTabs.forEach(t => t.classList.remove('active'));
            this.classList.add('active');
            currentCategory = this.dataset.category;
            currentPage = 1;
            hasMore = true;
            taskList.innerHTML = '';
            loadMoreTasks();
        });
    });

    // Search with debounce
    let searchTimeout;
    if (searchInput) {
        searchInput.addEventListener('input', function() {
            clearTimeout(searchTimeout);
            searchTimeout = setTimeout(() => {
                currentKeyword = this.value.trim();
                currentPage = 1;
                hasMore = true;
                taskList.innerHTML = '';
                loadMoreTasks();
            }, 500);
        });
    }

    // Load more tasks
    async function loadMoreTasks() {
        if (isLoading || !hasMore) return;

        isLoading = true;
        loadingMore.style.display = 'block';
        emptyState.style.display = 'none';

        try {
            const result = await loadTasks(currentPage, currentCategory, currentKeyword);

            if (result && result.data && result.data.data) {
                const tasks = result.data.data;

                if (tasks.length === 0) {
                    if (currentPage === 1) {
                        emptyState.style.display = 'block';
                    }
                    hasMore = false;
                } else {
                    tasks.forEach(task => {
                        taskList.appendChild(createTaskCard(task));
                    });
                    currentPage++;
                    hasMore = tasks.length >= TASKS_PER_PAGE;
                }
            }
        } catch (error) {
            console.error('Load tasks failed:', error);
            if (currentPage === 1) {
                showToast('加载失败，请重试', 'error');
            }
        } finally {
            isLoading = false;
            loadingMore.style.display = 'none';
        }
    }

    // Create task card element
    function createTaskCard(task) {
        const card = document.createElement('div');
        card.className = 'mobile-card task-card';
        card.dataset.taskId = task.id;

        const coverImage = task.cover_image || '/static/images/task-placeholder.svg';
        const price = parseFloat(task.unit_price || 0).toFixed(2);
        const remaining = task.remaining_count || 0;

        // Map task type to display text
        const typeMap = {
            'video': '视频',
            'image': '图文',
            'live': '直播'
        };
        const typeText = typeMap[task.type] || '视频';

        card.innerHTML = `
            <div class="task-card-image">
                <img src="${escapeHtml(coverImage)}" alt="${escapeHtml(task.title)}" loading="lazy">
                <div class="task-card-price">¥${price}</div>
            </div>
            <div class="task-card-content">
                <h3 class="task-card-title">${escapeHtml(task.title)}</h3>
                <div class="task-card-tags">
                    <span class="mobile-tag">${escapeHtml(typeText)}</span>
                    <span class="mobile-tag">${remaining === 0 ? '已满' : '剩余' + remaining}</span>
                </div>
                <div class="task-card-publisher">
                    <img src="/static/images/avatar-default.jpg" alt="商家" class="publisher-avatar">
                    <span class="publisher-name">${escapeHtml(task.publisher?.username || '商家')}</span>
                </div>
            </div>
        `;

        card.addEventListener('click', () => {
            window.location.href = `/mobile/task/${task.id}`;
        });

        return card;
    }

    // Pull-to-refresh
    initPullToRefresh(async () => {
        if (isLoading) return; // Prevent concurrent requests
        currentPage = 1;
        hasMore = true;
        taskList.innerHTML = '';
        await loadMoreTasks();
    });

    // Infinite scroll
    let scrollTimeout;
    window.addEventListener('scroll', () => {
        clearTimeout(scrollTimeout);
        scrollTimeout = setTimeout(() => {
            if (isLoading || !hasMore) return;

            const scrollTop = window.pageYOffset || document.documentElement.scrollTop;
            const windowHeight = window.innerHeight;
            const documentHeight = document.documentElement.scrollHeight;

            // Trigger load when 200px from bottom
            if (scrollTop + windowHeight >= documentHeight - 200) {
                loadMoreTasks();
            }
        }, 100);
    });

    // Initial load (skip if server already rendered tasks)
    if (taskList.children.length === 0) {
        loadMoreTasks();
    } else {
        // Server rendered initial tasks, prepare for page 2
        currentPage = 2;
    }
}

// 显示错误状态
function showErrorState(message = '加载失败') {
    const errorState = document.getElementById('errorState');
    const taskList = document.getElementById('taskList');
    const emptyState = document.getElementById('emptyState');

    if (taskList) taskList.style.display = 'none';
    if (emptyState) emptyState.style.display = 'none';
    if (errorState) {
        errorState.style.display = 'block';
        const textEl = errorState.querySelector('.mobile-empty-text');
        if (textEl) textEl.textContent = message;
    }
}

// 隐藏错误状态
function hideErrorState() {
    const errorState = document.getElementById('errorState');
    const taskList = document.getElementById('taskList');

    if (errorState) errorState.style.display = 'none';
    if (taskList) taskList.style.display = 'grid';
}

// Load approved works from API
async function loadWorks(page = 1, sort = 'created_at') {
    const params = new URLSearchParams({
        page: page,
        limit: 20,
        sort: sort
    });

    const data = await apiRequest(`/api/v1/works?${params}`);
    return data;
}

// Initialize works page
function initWorksPage() {
    const worksList = document.getElementById('worksList');
    const sortTabs = document.querySelectorAll('.mobile-sort-tabs .mobile-tag');
    const loadingMore = document.getElementById('loadingMore');
    const emptyState = document.getElementById('emptyState');

    if (!worksList) return; // Not on works page

    let currentPage = 1;
    let currentSort = 'created_at';
    let isLoading = false;
    let hasMore = true;

    // Sort filter
    sortTabs.forEach(tab => {
        tab.addEventListener('click', function() {
            sortTabs.forEach(t => t.classList.remove('active'));
            this.classList.add('active');
            currentSort = this.dataset.sort;
            currentPage = 1;
            hasMore = true;
            worksList.innerHTML = '';
            loadMoreWorks();
        });
    });

    // Load more works
    async function loadMoreWorks() {
        if (isLoading || !hasMore) return;

        isLoading = true;
        loadingMore.style.display = 'block';
        emptyState.style.display = 'none';

        try {
            const result = await loadWorks(currentPage, currentSort);

            if (result && result.data && result.data.data) {
                const works = result.data.data;

                if (works.length === 0) {
                    if (currentPage === 1) {
                        emptyState.style.display = 'block';
                    }
                    hasMore = false;
                } else {
                    works.forEach(work => {
                        worksList.appendChild(createWorkCard(work));
                    });
                    currentPage++;
                    hasMore = works.length >= 20;
                }
            }
        } catch (error) {
            console.error('Load works failed:', error);
            if (currentPage === 1) {
                showToast('加载失败，请重试', 'error');
            }
        } finally {
            isLoading = false;
            loadingMore.style.display = 'none';
        }
    }

    // Create work card element
    function createWorkCard(work) {
        const card = document.createElement('div');
        card.className = 'mobile-card work-card';
        card.dataset.workId = work.id;

        // Use placeholder for cover image (actual cover not in API response)
        const coverImage = '/static/images/task-placeholder.svg';
        const title = work.content || '作品';
        const creatorAvatar = work.creator_avatar || '/static/images/avatar-default.jpg';
        const creatorName = work.creator_name || '匿名';

        card.innerHTML = `
            <div class="work-card-image">
                <img src="${escapeHtml(coverImage)}" alt="${escapeHtml(title)}" loading="lazy">
            </div>
            <div class="work-card-content">
                <h3 class="work-card-title">${escapeHtml(title)}</h3>
                <div class="work-card-creator">
                    <img src="${escapeHtml(creatorAvatar)}" alt="${escapeHtml(creatorName)}" class="creator-avatar">
                    <span class="creator-name">${escapeHtml(creatorName)}</span>
                </div>
            </div>
        `;

        card.addEventListener('click', () => {
            window.location.href = `/mobile/work/${work.id}`;
        });

        return card;
    }

    // Infinite scroll
    let scrollTimeout;
    window.addEventListener('scroll', () => {
        clearTimeout(scrollTimeout);
        scrollTimeout = setTimeout(() => {
            if (isLoading || !hasMore) return;

            const scrollTop = window.pageYOffset || document.documentElement.scrollTop;
            const windowHeight = window.innerHeight;
            const documentHeight = document.documentElement.scrollHeight;

            // Trigger load when 200px from bottom
            if (scrollTop + windowHeight >= documentHeight - 200) {
                loadMoreWorks();
            }
        }, 100);
    });

    // Initial load (skip if server already rendered works)
    if (worksList.children.length === 0) {
        loadMoreWorks();
    } else {
        // Server rendered initial works, prepare for page 2
        currentPage = 2;
    }
}

// 页面初始化
document.addEventListener('DOMContentLoaded', function() {
    // 初始化底部导航栏
    initTabBar();

    // 初始化图片懒加载
    initLazyLoad();

    // 阻止双击缩放
    let lastTouchEnd = 0;
    document.addEventListener('touchend', function(event) {
        const now = Date.now();
        if (now - lastTouchEnd <= 300) {
            event.preventDefault();
        }
        lastTouchEnd = now;
    }, false);

    // 阻止长按菜单
    document.addEventListener('contextmenu', function(e) {
        e.preventDefault();
    });
});
