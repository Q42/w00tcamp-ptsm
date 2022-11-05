import * as functions from "firebase-functions";
import * as express from "express";
import { FieldValue } from "firebase-admin/firestore";
import { firestore } from ".";

const expressApp = express();

// const productId = 'prod_MjvCZGLYUzcXZy';
// const priceId = 'price_0M0RN72xzq6B1qO3DSWkTrn9';
// app.use(cors());

expressApp.post(
  "/",
  express.json({ type: "application/json" }),
  async (req, res) => {
    const event = req.body as any;

    functions.logger.info("Got stripe webhook call", {
      event,
    });

    // Handle the event
    switch (event.type) {
      case "payment_intent.succeeded":
        const paymentIntent = event.data.object;
        // Then define and call a method to handle the successful payment intent.
        await handlePaymentIntentSucceeded(paymentIntent);
        break;
      case "payment_method.attached":
        // const paymentMethod = event.data.object;
        // Then define and call a method to handle the successful attachment of a PaymentMethod.
        // handlePaymentMethodAttached(paymentMethod);
        break;
      // ... handle other event types
      default:
        functions.logger.info(`Unhandled event type ${event.type}`);
    }

    // Return a response to acknowledge receipt of the event
    res.json({ received: true });
  }
);

export const stripeWebhook = functions
  .region("europe-west1")
  .https.onRequest(expressApp);

const handlePaymentIntentSucceeded = async (paymentIntent: any) => {
  // a payment has been done, let's find out who did it
  // const paymentLink = paymentIntent.url as string;
  // const docs = await firestore
  //   .collection("balance")
  //   .where("paymentLink", "==", paymentLink)
  //   .get();
  // if (docs.size !== 1) {
  //   throw new Error(
  //     "Found too little or too many balance accounts with this paymentLink: " +
  //       paymentLink
  //   );
  // }
  // const userId = docs.docs[0].id;
  const userId = paymentIntent.receipt_email;

  const amount = parseInt(paymentIntent.amount, 10);
  const transactionId = paymentIntent.id;

  // add transaction
  await firestore.runTransaction(async (t) => {
    const balanceRef = firestore.collection("balance").doc(userId);
    const transactionRef = balanceRef
      .collection("transactions")
      .doc(transactionId);

    const transactionDoc = await t.get(transactionRef);
    if (transactionDoc.exists) {
      functions.logger.info("Got this transaction already!", {
        transactionId,
        userId,
        amount,
      });
      return;
    }

    const balanceDoc = await t.get(balanceRef);
    if (!balanceDoc.exists) {
      t.set(balanceRef, {
        balance: amount,
        added: FieldValue.serverTimestamp(),
        paymentLink: "https://buy.stripe.com/test_aEU9CI5K5bnrbKMdQQ",
      });
    } else {
      t.update(balanceRef, {
        balance: FieldValue.increment(amount),
      });
    }

    t.set(transactionRef, {
      added: FieldValue.serverTimestamp(),
      plus: amount,
      source: "stripe",
      paymentIntent,
    });
  });

  functions.logger.info("Transaction added!", {
    transactionId,
    userId,
    amount,
  });
};
