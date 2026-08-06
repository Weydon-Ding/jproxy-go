// jproxy-go Admin Console - Examples Management
(function() {
  'use strict';
  var UI = window.JProxyUI;

  var examplePages = { sonarr: 1, radarr: 1 };
  var pageSize = 20;

  async function loadExamples() {
    loadDomainExamples('sonarr');
    loadDomainExamples('radarr');
  }

  async function loadDomainExamples(domain) {
    var page = examplePages[domain] || 1;
    try {
      var result = await UI.api('/' + domain + '/example/query?' + UI.buildPageParams(page, pageSize));
      var container = document.getElementById(domain + '-examples');
      if (!container) return;

      var fragment = document.createDocumentFragment();

      if (result && Array.isArray(result.list)) {
        var list = UI.createElement('ul', { className: 'example-list' });
        result.list.forEach(function(example) {
          var li = UI.createElement('li', { className: 'example-item' });

          var textEl = UI.createElement('pre', {
            className: 'example-text',
            textContent: example.originalText || ''
          });
          li.appendChild(textEl);

          if (example.formatText) {
            var formatEl = UI.createElement('pre', {
              className: 'example-text',
              textContent: example.formatText
            });
            li.appendChild(formatEl);
          }

          var statusEl = UI.createElement('span', {
            className: 'status ' + (example.validStatus === 1 ? 'status-success' : 'status-error'),
            textContent: example.validStatus === 1 ? '有效' : '无效'
          });
          li.appendChild(statusEl);

          var removeBtn = UI.createElement('button', {
            className: 'btn btn-sm btn-danger',
            dataset: { hash: example.hash },
            textContent: '删除'
          });
          removeBtn.addEventListener('click', function() {
            removeExample(domain, example.hash);
          });
          li.appendChild(removeBtn);

          list.appendChild(li);
        });
        fragment.appendChild(list);

        var pagination = UI.createElement('div', { className: 'pagination' });
        var prevBtn = UI.createElement('button', {
          className: 'btn',
          textContent: '上一页'
        });
        prevBtn.disabled = page <= 1;
        prevBtn.addEventListener('click', function() {
          if (examplePages[domain] > 1) {
            examplePages[domain]--;
            loadDomainExamples(domain);
          }
        });

        var pageInfo = UI.createElement('span', {
          textContent: '第 ' + result.current + ' 页 / 共 ' + result.total + ' 条'
        });

        var nextBtn = UI.createElement('button', {
          className: 'btn',
          textContent: '下一页'
        });
        nextBtn.disabled = page * pageSize >= result.total;
        nextBtn.addEventListener('click', function() {
          examplePages[domain]++;
          loadDomainExamples(domain);
        });

        pagination.appendChild(prevBtn);
        pagination.appendChild(pageInfo);
        pagination.appendChild(nextBtn);
        fragment.appendChild(pagination);
      } else {
        fragment.appendChild(UI.createElement('p', { textContent: '暂无示例' }));
      }

      container.replaceChildren(fragment);
    } catch (error) {
      console.error('Failed to load examples:', error);
      var container = document.getElementById(domain + '-examples');
      if (container) {
        container.replaceChildren(UI.createElement('p', {
          className: 'status status-error',
          textContent: '加载失败'
        }));
      }
    }
  }

  async function removeExample(domain, hash) {
    if (!confirm('确定要删除这个示例吗？')) return;
    try {
      await UI.api('/' + domain + '/example/remove', {
        method: 'POST',
        body: [hash]
      });
      UI.showStatus('示例已删除');
      loadDomainExamples(domain);
    } catch (error) {
      UI.showStatus('删除失败', true);
    }
  }

  function setupExampleSave() {
    ['sonarr', 'radarr'].forEach(function(domain) {
      var form = document.getElementById(domain + '-example-form');
      if (!form) return;

      form.addEventListener('submit', async function(e) {
        e.preventDefault();
        var textarea = form.querySelector('textarea');
        if (!textarea || !textarea.value) return;

        try {
          await UI.api('/' + domain + '/example/save', {
            method: 'POST',
            body: { originalText: textarea.value }
          });
          UI.showStatus('示例已保存');
          textarea.value = '';
          loadDomainExamples(domain);
        } catch (error) {
          UI.showStatus('保存失败', true);
        }
      });
    });
  }

  UI.loadExamples = loadExamples;
  UI.setupExampleSave = setupExampleSave;

})();
