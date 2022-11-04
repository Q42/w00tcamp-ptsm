<template>
  <section>
    <img :src="logo" />
  </section>
  <section>
    <h3>Welcome</h3>
    <p>Your time is valuable! Make them pay for it :)</p>
    <v-btn @click="login">
      Log in
    </v-btn>
  </section>
</template>

<script setup lang="ts">
import { GoogleAuthProvider, signInWithPopup } from 'firebase/auth';

import logo from '../assets/ptsm-logo.jpeg';
import { auth } from '../stores/db';

async function login() {
  try {
    await signInWithPopup(auth, new GoogleAuthProvider());
  } catch (err) {
    console.error('Error during signInWithPopup', err);
    alert('inloggen mislukt');
  }
}
</script>
