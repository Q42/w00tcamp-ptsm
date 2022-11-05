<script setup lang="ts">
import { auth } from '../stores/db';
import { onMounted, ref } from 'vue';

const token = ref<string | null>(null)

onMounted(async () => {
  token.value = await auth.currentUser?.getIdToken() ?? null;
})
</script>

<template>
  <main>
    <v-card
      elevation="2"
    >
    <v-card-title>
      Configure on iOS
    </v-card-title> 
    <v-card-text>
      Download an authenticated mobile configuration file that you can use to provision your iOS device with the correct IMAP and SMTP settings.
    </v-card-text>
    <v-card-actions>
      <form target="_blank" method="POST" action="https://mail.pay2mail.me/provision">
        <input type="hidden" name="authorization" :value="token" />
        <v-btn type="submit" :disabled="!token">Download</v-btn>
      </form>
    </v-card-actions>
    </v-card>
  </main>
</template>
