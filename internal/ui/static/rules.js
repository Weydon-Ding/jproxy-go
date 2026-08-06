// jproxy-go Admin Console - Rules Management
(function() {
  'use strict';
  var UI = window.JProxyUI;

  var rulePages = { sonarr: 1, radarr: 1 };
  var pageSize = 20;

  async function loadRules(domain) {
    var page = rulePages[domain] || 1;
    try {
      var result = await UI.api('/' + domain + '/rule/query?' + UI.buildPageParams(page, pageSize));
      var tbody = document.querySelector('#' + domain + '-rules-table tbody');
      if (!tbody) return;

      var fragment = document.createDocumentFragment();

      if (result && Array.isArray(result.list)) {
        result.list.forEach(function(rule) {
          var tr = UI.createElement('tr');

          var tdToken = UI.createElement('td', { textContent: rule.token || '' });
          tr.appendChild(tdToken);

          var tdRemark = UI.createElement('td', { textContent: rule.remark || '' });
          tr.appendChild(tdRemark);

          var statusText = rule.validStatus === 1 ? '启用' : '禁用';
          var tdStatus = UI.createElement('td', { textContent: statusText });
          tr.appendChild(tdStatus);

          var tdActions = UI.createElement('td');
          var isEnabled = rule.validStatus === 1;
          var toggleBtn = UI.createElement('button', {
            className: 'btn btn-sm ' + (isEnabled ? 'btn-danger' : 'btn-primary'),
            dataset: { action: 'toggle', id: rule.id, next: isEnabled ? '0' : '1' },
            textContent: isEnabled ? '禁用' : '启用'
          });
          var removeBtn = UI.createElement('button', {
            className: 'btn btn-sm btn-danger',
            dataset: { action: 'remove', id: rule.id },
            textContent: '删除'
          });
          tdActions.appendChild(toggleBtn);
          tdActions.appendChild(removeBtn);
          tr.appendChild(tdActions);

          fragment.appendChild(tr);
        });

        var pageInfo = document.getElementById(domain + '-page-info');
        if (pageInfo) {
          pageInfo.textContent = '第 ' + result.current + ' 页 / 共 ' + result.total + ' 条';
        }

        var prevBtn = document.getElementById(domain + '-prev-btn');
        var nextBtn = document.getElementById(domain + '-next-btn');
        if (prevBtn) prevBtn.disabled = page <= 1;
        if (nextBtn) nextBtn.disabled = page * pageSize >= result.total;
      }

      tbody.replaceChildren(fragment);

      tbody.querySelectorAll('button').forEach(function(btn) {
        btn.addEventListener('click', function() {
          var action = btn.dataset.action;
          var id = btn.dataset.id;
          if (action === 'toggle') {
            var nextEnabled = btn.dataset.next === '1';
            toggleRule(domain, id, nextEnabled);
          }
          if (action === 'remove') removeRule(domain, id);
        });
      });
    } catch (error) {
      console.error('Failed to load rules:', error);
    }
  }

  async function toggleRule(domain, id, nextEnabled) {
    var endpoint = nextEnabled ? '/enable' : '/disable';
    try {
      await UI.api('/' + domain + '/rule' + endpoint, {
        method: 'POST',
        body: [id]
      });
      UI.showStatus('规则已更新');
      loadRules(domain);
    } catch (error) {
      UI.showStatus('操作失败', true);
    }
  }

  async function removeRule(domain, id) {
    if (!confirm('确定要删除这条规则吗？')) return;
    try {
      await UI.api('/' + domain + '/rule/remove', {
        method: 'POST',
        body: [id]
      });
      UI.showStatus('规则已删除');
      loadRules(domain);
    } catch (error) {
      UI.showStatus('删除失败', true);
    }
  }

  function setupRuleSync() {
    ['sonarr', 'radarr'].forEach(function(domain) {
      var btn = document.getElementById(domain + '-sync-btn');
      if (btn) {
        btn.addEventListener('click', async function() {
          try {
            await UI.api('/' + domain + '/rule/sync', { method: 'POST' });
            UI.showStatus(domain === 'sonarr' ? 'Sonarr 规则同步完成' : 'Radarr 规则同步完成');
            loadRules(domain);
          } catch (error) {
            UI.showStatus('同步失败', true);
          }
        });
      }
    });
  }

  function setupRulePagination() {
    ['sonarr', 'radarr'].forEach(function(domain) {
      var prevBtn = document.getElementById(domain + '-prev-btn');
      var nextBtn = document.getElementById(domain + '-next-btn');

      if (prevBtn) {
        prevBtn.addEventListener('click', function() {
          if (rulePages[domain] > 1) {
            rulePages[domain]--;
            loadRules(domain);
          }
        });
      }

      if (nextBtn) {
        nextBtn.addEventListener('click', function() {
          rulePages[domain]++;
          loadRules(domain);
        });
      }
    });
  }

  function setupRuleAdd() {
    ['sonarr', 'radarr'].forEach(function(domain) {
      var form = document.getElementById(domain + '-rule-form');
      if (!form) return;

      form.addEventListener('submit', async function(e) {
        e.preventDefault();

        var id = document.getElementById(domain + '-rule-id').value.trim();
        var token = document.getElementById(domain + '-rule-token').value.trim();
        var priority = parseInt(document.getElementById(domain + '-rule-priority').value || '0', 10);
        var regex = document.getElementById(domain + '-rule-regex').value;
        var replacement = document.getElementById(domain + '-rule-replacement').value;
        var offset = parseInt(document.getElementById(domain + '-rule-offset').value || '0', 10);
        var example = document.getElementById(domain + '-rule-example').value;
        var remark = document.getElementById(domain + '-rule-remark').value.trim();
        var validStatus = parseInt(document.getElementById(domain + '-rule-valid-status').value, 10);

        if (!token || !regex || !example) {
          UI.showStatus('Token、Regex、Example 必填', true);
          return;
        }

        var payload = {
          id: id || undefined,
          token: token,
          priority: priority,
          regex: regex,
          replacement: replacement,
          offset: offset,
          example: example,
          validStatus: validStatus
        };

        if (remark) payload.remark = remark;

        try {
          await UI.api('/' + domain + '/rule/save', {
            method: 'POST',
            body: payload
          });
          UI.showStatus('规则已保存');
          form.reset();
          loadRules(domain);
        } catch (error) {
          UI.showStatus('保存失败', true);
        }
      });
    });
  }

  UI.loadRules = loadRules;
  UI.setupRuleSync = setupRuleSync;
  UI.setupRulePagination = setupRulePagination;
  UI.setupRuleAdd = setupRuleAdd;

})();
