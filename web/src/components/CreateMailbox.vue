<template>
  <section>
    <h3>Hi there, first-timer!</h3>
    <p>Here you can create your PTSM mailbox.</p>
    <v-form
      ref="form"
      v-model="valid"
      lazy-validation
      @submit.prevent="submit"
    >
      <v-responsive
          class="mx-auto my-10"
        >
          <v-text-field
            v-model="name"
            label="Email prefix"
            required
            suffix="@pay2mail.me"
            hint="Must be unique, and [a-z0-9_]"
          />
        </v-responsive>
      <v-btn>
        Make it happen!
      </v-btn>
      <p class="my-5">{{ msg }}</p>
    </v-form>
  </section>
</template>

<script setup lang="ts">
import { doc, getDoc, setDoc } from '@firebase/firestore';
import { GoogleAuthProvider, signInWithPopup } from 'firebase/auth';
import { ref } from 'vue';

import logo from '../assets/ptsm-logo.jpeg';
import { auth, db } from '../stores/db';
import { email } from '../stores/user';

const valid = ref(true)
const name = ref('')
const msg = ref('')
const form = ref<any>(null);

async function submit() {
  console.log('got name', name.value);
  msg.value = '';

  if (!/^[a-z0-9_]+$/.test(name.value)) {
    msg.value = 'Invalid mailaddress, only use a-z, 0-9 and _';
    return;
  }

  const ref = doc(db, 'mailboxes', name.value + '@pay2mail.me');
  const existing = await getDoc(ref)
  if (existing.exists()) {
    msg.value = 'This address already exists! Try another one...';
  } else {
    setDoc(ref, {user: email.value});
  }
}
</script>

<style scoped>
.v-text-field__suffix {
  color: grey
}
</style>