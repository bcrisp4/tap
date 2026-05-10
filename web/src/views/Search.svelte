<script lang="ts">
  import Sidebar from '../components/Sidebar.svelte';
</script>

<div class="layout">
  <Sidebar />
  <main class="main"><p class="empty">Search coming in M9.</p></main>
</div>

<style>
  .layout { display: flex; height: 100vh; }
  .main { flex: 1; display: flex; align-items: center; justify-content: center; background: var(--bg); }
  .empty { color: var(--ink-3); font-family: var(--mono); font-size: 11px; }
</style>
