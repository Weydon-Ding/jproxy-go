// jproxy-go Admin Console - Core utilities
// Shared namespace: JProxyUI

(function(exports) {
  'use strict';

  // API call function - returns raw response data
  async function api(path, options) {
    options = options || {};
    var token = sessionStorage.getItem('token');
    var headers = options.headers || {};
    headers['Content-Type'] = 'application/json';

    if (token) {
      headers['Authorization'] = 'Bearer ' + token;
    }

    try {
      var response = await fetch('/api' + path, {
        method: options.method || 'GET',
        headers: headers,
        body: options.body ? JSON.stringify(options.body) : undefined
      });

      if (response.status === 401 || response.status === 403) {
        sessionStorage.removeItem('token');
        return null;
      }

      if (!response.ok) {
        throw new Error('请求失败');
      }

      var contentType = response.headers.get('content-type');
      if (contentType && contentType.includes('application/json')) {
        return response.json();
      }
      return response.text();
    } catch (error) {
      console.error('API error:', error);
      throw error;
    }
  }

  // Show status message
  function showStatus(message, isError) {
    var statusEl = document.getElementById('status-message');
    if (!statusEl) return;
    statusEl.textContent = message;
    statusEl.className = 'status ' + (isError ? 'status-error' : 'status-success');
    statusEl.classList.remove('hidden');
    setTimeout(function() {
      statusEl.classList.add('hidden');
    }, 5000);
  }

  // Create element with attributes and text
  function createElement(tag, attrs, text) {
    var el = document.createElement(tag);
    if (attrs) {
      Object.keys(attrs).forEach(function(key) {
        if (key === 'className') {
          el.className = attrs[key];
        } else if (key === 'textContent') {
          el.textContent = attrs[key];
        } else if (key === 'dataset') {
          Object.keys(attrs.dataset).forEach(function(dk) {
            el.dataset[dk] = attrs.dataset[dk];
          });
        } else {
          el.setAttribute(key, attrs[key]);
        }
      });
    }
    if (text !== undefined && text !== null) {
      el.textContent = text;
    }
    return el;
  }

  // Parse query params for pagination
  function buildPageParams(current, pageSize) {
    return 'current=' + encodeURIComponent(current) + '&pageSize=' + encodeURIComponent(pageSize);
  }

  exports.api = api;
  exports.showStatus = showStatus;
  exports.createElement = createElement;
  exports.buildPageParams = buildPageParams;

})(window.JProxyUI = window.JProxyUI || {});
