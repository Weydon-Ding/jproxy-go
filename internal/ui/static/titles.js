// jproxy-go Admin Console - Titles Management
(function() {
  'use strict';
  var UI = window.JProxyUI;
  var titlePages = { sonarr: 1, radarr: 1, tmdb: 1 };
  var pageSize = 20;

  function loadTitles() { loadSonarrTitles(); loadRadarrTitles(); loadTmdbTitles(); }
  function loadSonarrTitles() { loadTitleDomain('sonarr', 'sonarr-titles-table', 'sonarr-page-info', 'sonarr-prev-btn', 'sonarr-next-btn'); }
  function loadRadarrTitles() { loadTitleDomain('radarr', 'radarr-titles-table', 'radarr-page-info', 'radarr-prev-btn', 'radarr-next-btn'); }
  function loadTmdbTitles() { loadTitleDomain('tmdb', 'tmdb-titles-table', 'tmdb-page-info', 'tmdb-prev-btn', 'tmdb-next-btn'); }

  function loadTitleDomain(domain, tableId, pageInfoId, prevBtnId, nextBtnId) {
    var page = titlePages[domain] || 1;
    UI.api('/' + domain + '/title/query?' + UI.buildPageParams(page, pageSize)).then(function(result) {
      var table = document.getElementById(tableId);
      if (!table) return;
      var tbody = table.querySelector('tbody');
      if (!tbody) return;
      var fragment = document.createDocumentFragment();
      if (result && Array.isArray(result.list)) {
        result.list.forEach(function(title) {
          var tr = UI.createElement('tr');
          var tdCheck = UI.createElement('td');
          tdCheck.appendChild(UI.createElement('input', { type: 'checkbox', dataset: { id: String(title.id) } }));
          tr.appendChild(tdCheck);
          if (domain === 'sonarr') {
            tr.appendChild(UI.createElement('td', { textContent: title.mainTitle || '' }));
            tr.appendChild(UI.createElement('td', { textContent: title.title || '' }));
            tr.appendChild(UI.createElement('td', { textContent: String(title.seasonNumber || '') }));
          } else if (domain === 'radarr') {
            tr.appendChild(UI.createElement('td', { textContent: title.mainTitle || '' }));
            tr.appendChild(UI.createElement('td', { textContent: title.title || '' }));
            tr.appendChild(UI.createElement('td', { textContent: String(title.year || '') }));
          } else {
            tr.appendChild(UI.createElement('td', { textContent: title.title || '' }));
            tr.appendChild(UI.createElement('td', { textContent: String(title.tvdbId || '') }));
          }
          var tdActions = UI.createElement('td');
          var removeBtn = UI.createElement('button', { className: 'btn btn-sm btn-danger', dataset: { id: String(title.id) }, textContent: '删除' });
          removeBtn.addEventListener('click', function() { removeTitle(domain, title.id); });
          tdActions.appendChild(removeBtn);
          tr.appendChild(tdActions);
          fragment.appendChild(tr);
        });
        var pageInfo = document.getElementById(pageInfoId);
        if (pageInfo) pageInfo.textContent = '第 ' + result.current + ' 页 / 共 ' + result.total + ' 条';
        var prevBtn = document.getElementById(prevBtnId);
        var nextBtn = document.getElementById(nextBtnId);
        if (prevBtn) prevBtn.disabled = page <= 1;
        if (nextBtn) nextBtn.disabled = page * pageSize >= result.total;
        var selectAll = table.querySelector('thead input[type="checkbox"]');
        if (selectAll) { selectAll.checked = false; selectAll.indeterminate = false; }
      }
      tbody.replaceChildren(fragment);
      var selectAll = table.querySelector('thead input[type="checkbox"]');
      if (selectAll) {
        selectAll.onchange = function() {
          tbody.querySelectorAll('input[type="checkbox"]').forEach(function(cb) { cb.checked = selectAll.checked; });
          updateBatchButtonState(domain);
        };
      }
      tbody.querySelectorAll('input[type="checkbox"]').forEach(function(cb) {
        cb.onchange = function() { updateSelectAllState(tbody, selectAll); updateBatchButtonState(domain); };
      });
      updateBatchButtonState(domain);
    }).catch(function(error) { console.error('Failed to load ' + domain + ' titles:', error); });
  }

  function updateSelectAllState(tbody, selectAll) {
    if (!selectAll) return;
    var checkboxes = tbody.querySelectorAll('input[type="checkbox"]');
    var checkedCount = Array.from(checkboxes).filter(function(cb) { return cb.checked; }).length;
    selectAll.checked = checkedCount === checkboxes.length;
    selectAll.indeterminate = checkedCount > 0 && checkedCount < checkboxes.length;
  }

  function updateBatchButtonState(domain) {
    var btn = document.getElementById(domain + '-batch-delete');
    if (!btn) return;
    var table = document.getElementById(domain + '-titles-table');
    if (!table) return;
    btn.disabled = table.querySelectorAll('tbody input[type="checkbox"]:checked').length === 0;
  }

  function removeTitle(domain, id) {
    if (!confirm('确定要删除这个标题吗？')) return;
    UI.api('/' + domain + '/title/remove', { method: 'POST', body: [id] }).then(function() {
      UI.showStatus('标题已删除');
      loadTitleDomainAfterDelete(domain, id);
    }).catch(function() { UI.showStatus('删除失败', true); });
  }

  function loadTitleDomainAfterDelete(domain, deletedId) {
    var page = titlePages[domain];
    UI.api('/' + domain + '/title/query?' + UI.buildPageParams(page, pageSize)).then(function(result) {
      if (result && result.list && result.list.length === 0 && page > 1) {
        titlePages[domain] = page - 1;
      }
      if (domain === 'sonarr') loadSonarrTitles();
      else if (domain === 'radarr') loadRadarrTitles();
      else loadTmdbTitles();
    });
  }

  function batchDeleteTitles(domain, tableId) {
    var table = document.getElementById(tableId);
    if (!table) return;
    var checkboxes = table.querySelectorAll('tbody input[type="checkbox"]:checked');
    if (checkboxes.length === 0) { UI.showStatus('请先选择要删除的标题', true); return; }
    if (!confirm('确定要删除选中的 ' + checkboxes.length + ' 个标题吗？')) return;
    var ids = Array.from(checkboxes).map(function(cb) { return parseInt(cb.dataset.id, 10); });
    UI.api('/' + domain + '/title/remove', { method: 'POST', body: ids }).then(function() {
      UI.showStatus('已删除 ' + ids.length + ' 个标题');
      if (domain === 'sonarr') loadSonarrTitles();
      else if (domain === 'radarr') loadRadarrTitles();
      else loadTmdbTitles();
    }).catch(function() { UI.showStatus('删除失败', true); });
  }

  function setupTitleSync() {
    var syncMap = { 'sonarr-title-sync-btn': 'sonarr', 'radarr-title-sync-btn': 'radarr', 'tmdb-title-sync-btn': 'tmdb' };
    Object.keys(syncMap).forEach(function(btnId) {
      var btn = document.getElementById(btnId);
      if (btn) btn.addEventListener('click', function() {
        var domain = syncMap[btnId];
        UI.api('/' + domain + '/title/sync', { method: 'POST' }).then(function() {
          UI.showStatus((domain === 'sonarr' ? 'Sonarr' : domain === 'radarr' ? 'Radarr' : 'TMDB') + ' 同步完成');
          if (domain === 'sonarr') loadSonarrTitles();
          else if (domain === 'radarr') loadRadarrTitles();
          else loadTmdbTitles();
        }).catch(function() { UI.showStatus('同步失败', true); });
      });
    });
  }

  function setupTmdbSave() {
    var form = document.getElementById('tmdb-save-form');
    if (!form) return;
    form.addEventListener('submit', function(e) {
      e.preventDefault();
      var tvdbId = parseInt(document.getElementById('tmdb-tvdbid').value, 10);
      var tmdbIdRaw = document.getElementById('tmdb-tmdbid').value;
      var language = document.getElementById('tmdb-language').value || 'zh-CN';
      var title = document.getElementById('tmdb-title-input').value;
      var validStatus = parseInt(document.getElementById('tmdb-valid-status').value, 10) || 1;
      if (!tvdbId || !title) { UI.showStatus('TVDB ID 和标题必填', true); return; }
      var payload = { tvdbId: tvdbId, language: language, title: title, validStatus: validStatus };
      if (tmdbIdRaw) payload.tmdbId = parseInt(tmdbIdRaw, 10);
      UI.api('/tmdb/title/save', { method: 'POST', body: payload }).then(function() {
        UI.showStatus('TMDB 标题已保存');
        form.reset();
        loadTmdbTitles();
      }).catch(function() { UI.showStatus('保存失败', true); });
    });
  }

  function setupBatchDelete() {
    var deleteBtns = { 'sonarr-batch-delete': { domain: 'sonarr' }, 'radarr-batch-delete': { domain: 'radarr' }, 'tmdb-batch-delete': { domain: 'tmdb' } };
    Object.keys(deleteBtns).forEach(function(btnId) {
      var btn = document.getElementById(btnId);
      if (btn) btn.addEventListener('click', function() { batchDeleteTitles(deleteBtns[btnId].domain, deleteBtns[btnId].domain + '-titles-table'); });
    });
  }

  UI.loadTitles = loadTitles;
  UI.setupTitleSync = setupTitleSync;
  UI.setupTmdbSave = setupTmdbSave;
  UI.setupBatchDelete = setupBatchDelete;
})();
