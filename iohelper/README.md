# Buffering helpers

These are asynchronous buffers to help us deal with blocking APIs. They are similar
to what you would except a from a TCP transmit window. 

There are two asynchronous buffer implementations. Their respective performance remains
to be tested.

## AsyncWriterBuffer
- Writes to a bytes buffer
- Asynchronously write them
- In-transmission buffer size is precise

## AsyncWriterChannel
- Writes to a channel of bytes buffer
- Asynchronous write them
- In-transmission buffer size is estimated (we're not cutting write buffer into smaller chunks)
