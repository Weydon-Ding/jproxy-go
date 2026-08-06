// jproxy-go Admin Console - Main entry point
(function() {
  'use strict';
  var UI = window.JProxyUI;

  function setupTabs() {
    var tabs = document.querySelectorAll('.nav-tab');
    tabs.forEach(function(tab) {
      tab.addEventListener('click', function() {
        tabs.forEach(function(t) {
          t.classList.remove('active');
          t.setAttribute('aria-selected', 'false');
        });
        tab.classList.add('active');
        tab.setAttribute('aria-selected', 'true');

        var reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
        tab.scrollIntoView({ block: 'nearest', inline: 'nearest', behavior: reducedMotion ? 'auto' : 'smooth' });

        var tabName = tab.dataset.tab;
        var panels = document.querySelectorAll('.tab-content');
        panels.forEach(function(panel) {
          panel.classList.remove('active');
          panel.setAttribute('aria-hidden', 'true');
        });

        var activePanel = document.getElementById(tabName + '-tab');
        if (activePanel) {
          activePanel.classList.add('active');
          activePanel.setAttribute('aria-hidden', 'false');
        }

        if (tabName === 'sonarr-rule') UI.loadRules('sonarr');
        if (tabName === 'radarr-rule') UI.loadRules('radarr');
        if (tabName === 'examples') UI.loadExamples();
        if (tabName === 'titles') UI.loadTitles();
      });
    });
  }

  async function init() {
    UI.setupLoginForm();
    UI.setupLogout();
    UI.setupAccountUpdate();
    UI.setupConfigSave();
    UI.setupCacheClear();
    UI.setupRuleSync();
    UI.setupRulePagination();
    UI.setupRuleAdd();
    UI.setupExampleSave();
    UI.setupTitleSync();
    UI.setupTmdbSave();
    UI.setupBatchDelete();
    setupTabs();

    var token = sessionStorage.getItem('token');
    if (token) {
      await UI.showConsoleView();
    } else {
      var authOk = await UI.initAuth();
      if (authOk && sessionStorage.getItem('token')) {
        await UI.showConsoleView();
      } else {
        UI.showLoginView();
      }
    }
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
