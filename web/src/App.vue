<script setup lang="ts">
import { ref, watch } from 'vue';
import { RouterLink, RouterView } from 'vue-router'
import herman from './assets/herman.jpeg';
import Login from './components/Login.vue';
import CreateMailbox from './components/CreateMailbox.vue';
import { email, loading, mailboxId, logout } from './stores/user';

const drawer = ref(false);
</script>

<template>
  <v-layout>
    <template v-if="loading">
      <v-progress-circular indeterminate class="mx-auto" />
    </template>
    <template v-else-if="!email">
      <Login />
    </template>
    <template v-else-if="!mailboxId">
      <CreateMailbox />
    </template>
    <template v-else>
      <v-app-bar
        prominent
      >
        <v-app-bar-nav-icon variant="text" @click.stop="drawer = !drawer"></v-app-bar-nav-icon>

        <v-toolbar-title>PTSM - PayMail</v-toolbar-title>

        <v-spacer></v-spacer>

        <v-btn variant="text" icon="mdi-magnify"></v-btn>
        <v-btn variant="text" icon="mdi-filter"></v-btn>
        <v-btn variant="text" icon="mdi-dots-vertical"></v-btn>
      </v-app-bar>

      <v-navigation-drawer
        app="true"
        :permanent="drawer"
      >
        <v-list>
          <v-list-item
            :prepend-avatar="herman"
            :title="mailboxId.split('@')[0]"
            :subtitle="mailboxId"
          ></v-list-item>
        </v-list>

        <v-divider></v-divider>

        <v-list density="compact" nav>
          <v-list-item prepend-icon="mdi-inbox-arrow-down" title="Inbox" to="/inbox"></v-list-item>
          <v-list-item prepend-icon="mdi-cash-clock" title="Pending payment" to="/pending-payment"></v-list-item>
        </v-list>

        <v-divider></v-divider>

        <v-list density="compact" nav>
          <v-list-item prepend-icon="mdi-cog" title="Settings" to="/settings" />
          <v-list-item prepend-icon="mdi-logout" title="Log out" @click="logout" />
        </v-list>
      </v-navigation-drawer>

      <v-main>
        <RouterView />
      </v-main>
    </template>
  </v-layout>
</template>

<style scoped>

</style>
