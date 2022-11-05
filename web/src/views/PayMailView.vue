<script setup lang="ts">
  import { getDoc, doc } from '@firebase/firestore';
  import { onMounted, ref, type Ref } from 'vue';
  import {balance, transactions} from '../stores/balance'
  import { db } from '../stores/db';

  const props = defineProps({
    recipient: { type: String, required: true },
    emailId: { type: String, required: true },
  })
  const loading = ref(true);
  const mail: Ref<Record<string, string>> = ref({});
  onMounted(async () => {
    const d = await getDoc(doc(db, "mailboxes", `${props.recipient}@pay2mail.me`, 'emails', props.emailId))
    if (d.exists()) {
      mail.value = d.data() as Record<string, string>;
    } else {
      console.error('Cannot find email')
    }
    loading.value = false;
  })
</script>

<template>
  <template v-if="loading">
    <v-progress-circular indeterminate class="mx-auto" />
  </template>

  <template v-else-if="!mail.sender">
    <p>404 :(</p>
  </template>

  <v-card
    v-else
    elevation="2"
  >
    <v-card-title>
      Pay to deliver this e-mail
    </v-card-title> 
    <v-card-subtitle>
      From {{mail.sender}} to {{recipient}}@pay2mail.me
    </v-card-subtitle>
    <v-card-text>
      <p>{{recipient}} values his/her time, and would like to be compensated for spending it reading your mail.</p>
      <p>Delivering this mail will cost you €0.10.</p>
    </v-card-text>
    <v-card-actions>
      <v-btn target="_blank" :href="mail.paymentLink" :disabled="!mail.paymentLink">Pay</v-btn>
    </v-card-actions>
  </v-card>
</template>
