# Twilio

This package handles outgoing SMS, MMS and Voice and Video

Run the test using the valid U.S. number found in [https://www.receivesms.co/us-phone-number/3017/](https://www.receivesms.co/us-phone-number/3017/). This is like mailinator for emails but for SMS and MMS.

Make sure that you also use test numbers found in [https://www.twilio.com/docs/iam/test-credentials#test-sms-messages](https://www.twilio.com/docs/iam/test-credentials#test-sms-messages) - ensure it's not pushed to prod, as it doesn't actually send anything out, just test numbers that returns exceptions or not depending on your tests.