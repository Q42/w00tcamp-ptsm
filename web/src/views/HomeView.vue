<script setup lang="ts">
  import {balance, transactions} from '../stores/balance'
</script>

<template>
  <template v-if="!balance">
    <v-progress-circular indeterminate class="mx-auto" />
  </template>

  <v-card
    v-else
    elevation="2"
  >
    <v-card-title>
      Balance
    </v-card-title> 
    <v-card-text>
      You currently have <b>{{ (balance?.balance ?? 0) }}</b> credits.
    </v-card-text>
    <v-card-actions>
      <v-btn target="_blank" :href="balance?.paymentLink" :disabled="!balance?.paymentLink">Give me more!</v-btn>
    </v-card-actions>
  </v-card>
</template>
