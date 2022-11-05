import * as functions from "firebase-functions";
import { initializeApp } from "firebase-admin/app";
import { FieldValue, getFirestore } from "firebase-admin/firestore";

const app = initializeApp();
export const firestore = getFirestore(app);

export const helloWorld = functions
  .region("europe-west1")
  .https.onRequest((request, response) => {
    functions.logger.info("Hello logs!", { structuredData: true });
    response.send("Hello from Firebase!");
  });

export { stripeWebhook } from "./stripeWebhook";

/** user can build balance */
export const createUser = functions.auth.user().onCreate(async (user) => {
  const email = user.email;
  if (!email) {
    functions.logger.error("user created without email", { user });
    return;
  }
  await firestore.runTransaction(async (t) => {
    const ref = firestore.collection("balance").doc(email);
    const doc = await t.get(ref);
    if (!doc.exists) {
      t.set(ref, {
        balance: 0,
        added: FieldValue.serverTimestamp(),
        paymentLink: "https://buy.stripe.com/test_aEU9CI5K5bnrbKMdQQ",
      });
    }
  });
});
