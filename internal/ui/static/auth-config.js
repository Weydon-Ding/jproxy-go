// jproxy-go Admin Console - Auth and Config
(function() {
  'use strict';
  var UI = window.JProxyUI;

  var loginEnabled = null;
  var isAnonymous = false;
  var fullConfig = [];

  // Check login status and auto-login if disabled
  async function initAuth() {
    try {
      loginEnabled = await UI.api('/system/user/isLoginEnabled');
      if (!loginEnabled) {
        var result = await UI.api('/system/user/login', {
          method: 'POST',
          body: { username: '', password: '' }
        });
        if (result && typeof result === 'string') {
          sessionStorage.setItem('token', result);
          isAnonymous = true;
          return true;
        }
      }
      return !!loginEnabled;
    } catch (error) {
      console.error('Auth init failed:', error);
      return false;
    }
  }

  // Show login view
  function showLoginView() {
    var loginView = document.getElementById('login-view');
    var consoleView = document.getElementById('console-view');
    if (loginView) loginView.classList.remove('hidden');
    if (consoleView) consoleView.classList.add('hidden');
  }

  // Show console view
  async function showConsoleView() {
    var loginView = document.getElementById('login-view');
    var consoleView = document.getElementById('console-view');
    if (loginView) loginView.classList.add('hidden');
    if (consoleView) consoleView.classList.remove('hidden');

    try {
      var userInfo = await UI.api('/system/user/info');
      if (userInfo) {
        var userInfoEl = document.getElementById('user-info');
        if (userInfoEl) {
          userInfoEl.textContent = (userInfo.username || '用户') + (userInfo.role === 'ANONYMOUS' ? ' (匿名)' : '');
        }
        var accountUsername = document.getElementById('account-username');
        if (accountUsername && userInfo.username) {
          accountUsername.value = userInfo.username;
        }
        var isAnonymous = userInfo.role === 'ANONYMOUS';
        var accountAvailable = !isAnonymous && loginEnabled;
        var accountTab = document.getElementById('tab-account');
        if (accountTab) accountTab.style.display = accountAvailable ? 'block' : 'none';
        var accountSection = document.getElementById('account-section');
        if (accountSection) accountSection.style.display = accountAvailable ? 'block' : 'none';
      }
      loadSystemConfig();
    } catch (error) {
      console.error('Failed to load user info:', error);
    }
  }

  // Handle login form
  function setupLoginForm() {
    var form = document.getElementById('login-form');
    if (!form) return;
    form.addEventListener('submit', async function(e) {
      e.preventDefault();
      var username = document.getElementById('username').value;
      var password = document.getElementById('password').value;
      var statusEl = document.getElementById('login-status');

      try {
        var result = await UI.api('/system/user/login', {
          method: 'POST',
          body: { username: username, password: password }
        });
        if (result && typeof result === 'string') {
          sessionStorage.setItem('token', result);
          showConsoleView();
        }
      } catch (error) {
        if (statusEl) {
          statusEl.textContent = '登录失败';
          statusEl.className = 'status status-error';
          statusEl.classList.remove('hidden');
        }
      }
    });
  }

  // Handle logout
  function setupLogout() {
    var btn = document.getElementById('logout-btn');
    if (!btn) return;
    btn.addEventListener('click', async function() {
      try {
        await UI.api('/system/user/logout', { method: 'POST' });
      } catch (error) {
        console.error('Logout failed:', error);
      }
      sessionStorage.removeItem('token');
      showLoginView();
    });
  }

  // Handle account update
  function setupAccountUpdate() {
    var form = document.getElementById('account-form');
    if (!form) return;
    form.addEventListener('submit', async function(e) {
      e.preventDefault();
      var username = document.getElementById('account-username').value;
      var password = document.getElementById('account-password').value;
      var statusEl = document.getElementById('account-status');

      var payload = { username: username };
      if (password) {
        payload.password = password;
      }

      try {
        await UI.api('/system/user/update', {
          method: 'POST',
          body: payload
        });
        UI.showStatus('账户已更新');
        document.getElementById('account-password').value = '';
        showConsoleView();
      } catch (error) {
        if (statusEl) {
          statusEl.textContent = '更新失败';
          statusEl.className = 'status status-error';
          statusEl.classList.remove('hidden');
        }
        document.getElementById('account-password').value = '';
      }
    });
  }

  // Downloader config keys to hide from UI but preserve in payload
  var downloaderKeys = [
    'qbittorrentUrl', 'qbittorrentUsername', 'qbittorrentPassword',
    'transmissionUrl', 'transmissionPassword'
  ];

  // API key fields that should be password type
  var apiKeyFields = ['sonarrApikey', 'radarrApikey', 'tmdbApikey'];

  // Load system config
  async function loadSystemConfig() {
    try {
      var configs = await UI.api('/system/config/query');
      if (Array.isArray(configs)) {
        fullConfig = configs;
        renderConfigFields(configs);
      }
    } catch (error) {
      console.error('Failed to load config:', error);
    }
  }

  // Render config fields, hiding downloader rows
  function renderConfigFields(configs) {
    var container = document.getElementById('config-fields');
    if (!container) return;

    var fragment = document.createDocumentFragment();

    configs.forEach(function(config) {
      if (downloaderKeys.indexOf(config.key) !== -1) {
        return;
      }

      var group = UI.createElement('div', { className: 'form-group' });

      var label = UI.createElement('label', {
        htmlFor: 'config-' + config.id,
        textContent: config.key
      });
      group.appendChild(label);

      var isApiKey = apiKeyFields.indexOf(config.key) !== -1;
      var input = UI.createElement('input', {
        type: isApiKey ? 'password' : 'text',
        id: 'config-' + config.id,
        name: config.key,
        dataset: { id: String(config.id) }
      });
      input.value = config.value || '';
      group.appendChild(input);

      fragment.appendChild(group);
    });

    container.replaceChildren(fragment);
  }

  // Handle config save
  function setupConfigSave() {
    var form = document.getElementById('config-form');
    if (!form) return;
    form.addEventListener('submit', async function(e) {
      e.preventDefault();

      // Build full config payload preserving hidden rows
      var payload = fullConfig.map(function(config) {
        var input = document.getElementById('config-' + config.id);
        return {
          id: config.id,
          key: config.key,
          value: input ? input.value : (config.value || ''),
          validStatus: 1
        };
      });

      try {
        await UI.api('/system/config/update', {
          method: 'POST',
          body: payload
        });
        UI.showStatus('配置已保存');
      } catch (error) {
        UI.showStatus('保存失败', true);
      }
    });
  }

  // Cache clear buttons
  function setupCacheClear() {
    var clearBtn = document.getElementById('clear-cache-btn');
    if (clearBtn) {
      clearBtn.addEventListener('click', async function() {
        try {
          await UI.api('/system/cache/clear', {
            method: 'POST',
            body: { cacheName: 'result' }
          });
          UI.showStatus('缓存已清除');
        } catch (error) {
          UI.showStatus('清除缓存失败', true);
        }
      });
    }

    var clearAllBtn = document.getElementById('clear-all-cache-btn');
    if (clearAllBtn) {
      clearAllBtn.addEventListener('click', async function() {
        try {
          await UI.api('/system/cache/clearAll', { method: 'POST' });
          UI.showStatus('所有缓存已清除');
        } catch (error) {
          UI.showStatus('清除缓存失败', true);
        }
      });
    }
  }

  // Initialize auth module
  UI.initAuth = initAuth;
  UI.showLoginView = showLoginView;
  UI.showConsoleView = showConsoleView;
  UI.setupLoginForm = setupLoginForm;
  UI.setupLogout = setupLogout;
  UI.setupAccountUpdate = setupAccountUpdate;
  UI.setupConfigSave = setupConfigSave;
  UI.setupCacheClear = setupCacheClear;

})();
