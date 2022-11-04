import * as functions from "firebase-functions";
// import * as cors from "cors";
import * as express from "express";

const app = express();

// const productId = 'prod_MjvCZGLYUzcXZy';
// const priceId = 'price_0M0RN72xzq6B1qO3DSWkTrn9';
// app.use(cors());

app.post("/", express.json({ type: "application/json" }), async (req, res) => {
  const event = req.body as any;

  functions.logger.info("Got stripe webhook call", {
    event,
  });

  // Handle the event
  switch (event.type) {
    case "payment_intent.succeeded":
      const paymentIntent = event.data.object;
      // Then define and call a method to handle the successful payment intent.
      handlePaymentIntentSucceeded(paymentIntent);
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
});

export const stripeWebhook = functions
  .region("europe-west1")
  .https.onRequest(app);

const handlePaymentIntentSucceeded = (paymentIntent: any) => {};
