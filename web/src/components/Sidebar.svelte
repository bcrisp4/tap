<script lang="ts">
  import FeedAvatar from './FeedAvatar.svelte';
  import AddFeedForm from './AddFeedForm.svelte';
  import { subscriptions } from '../lib/store';
  // Sidebar is purely presentational — the parent view (Unread) owns the
  // subscriptions.load() call so the request fires once per page render.
</script>

<aside class="sidebar">
  <div class="brand">tap<span class="dot">.</span></div>

  <div class="group-title">READING</div>
  <a class="navitem active" href="/">Unread</a>

  <div class="group-title">FEEDS</div>
  {#each $subscriptions as sub (sub.id)}
    <div class="feedrow">
      <FeedAvatar feedURL={sub.feed_url} />
      <span class="feedname" title={sub.title}>{sub.title}</span>
    </div>
  {/each}

  <div class="group-title">SYSTEM</div>
  <AddFeedForm />
</aside>

<style>
  .sidebar {
    width: 240px;
    border-right: 1px solid var(--rule);
    background: var(--bg);
    height: 100vh;
    overflow-y: auto;
    flex: none;
  }
  .brand {
    font-family: var(--sans);
    font-weight: 600;
    font-size: 17px;
    padding: 18px 20px 8px;
  }
  .brand .dot {
    color: var(--accent);
    font-weight: 700;
    margin-left: 1px;
  }
  .group-title {
    font-family: var(--mono);
    font-size: 10px;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--ink-3);
    padding: 16px 20px 6px;
  }
  .navitem {
    display: block;
    padding: 6px 20px;
    font-family: var(--sans);
    font-size: 13px;
    color: var(--ink);
    border-left: 2px solid transparent;
  }
  .navitem.active { border-left-color: var(--accent); }
  .feedrow {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 20px;
  }
  .feedname {
    font-family: var(--sans);
    font-size: 12px;
    color: var(--ink-2);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
