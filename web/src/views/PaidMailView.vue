<script setup lang="ts">
  import { getDoc, doc, onSnapshot } from '@firebase/firestore';
import type { Unsubscribe } from '@firebase/util';
  import { onMounted, ref, type Ref } from 'vue';
  import {balance, transactions} from '../stores/balance'
  import { db } from '../stores/db';

  const props = defineProps({
    recipient: { type: String, required: true },
    emailId: { type: String, required: true },
  })
  const loading = ref(true);
  let cancel: Unsubscribe | undefined;
  onMounted(async () => {
    cancel = onSnapshot(doc(db, "mailboxes", `${props.recipient}@pay2mail.me`, 'emails', props.emailId), (doc) => {
      console.log('got doc', doc.exists());
      if (!doc.exists()) {
        cancel && cancel()
        loading.value = false;
      }
    })
  })
</script>

<template>
  <template v-if="loading">
    <p>Waiting for your payment...</p>
    <v-progress-circular indeterminate class="mx-auto" />
  </template>

  <v-card
    v-else
    elevation="2"
  >
    <v-card-title>
      Thnx! 🎉
    </v-card-title> 
    <!-- <v-card-subtitle>
    </v-card-subtitle> -->
    <v-card-text>
      Your mail has been delivered.
    </v-card-text>
  </v-card>
</template>
