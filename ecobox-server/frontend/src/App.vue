<template>
  <div id="app">
    <router-view />
  </div>
</template>

<script>
export default {
  name: 'App'
}
</script>

<style>
html {
  transition: background-color 0.2s ease, color 0.2s ease;
}

#app {
  min-height: 100vh;
  background-color: var(--bg-secondary);
  color: var(--text-primary);
  transition: background-color 0.2s ease, color 0.2s ease;
}
</style>